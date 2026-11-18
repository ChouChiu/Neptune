package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ChouChiu/neptune/internal/adminpanel"
	"github.com/ChouChiu/neptune/internal/bot"
	"github.com/ChouChiu/neptune/internal/db"
	"github.com/ChouChiu/neptune/internal/github"
	"github.com/ChouChiu/neptune/internal/model"
	tgbot "github.com/go-telegram/bot"
)

const maxGitHubWebhookBodyBytes = 5 << 20 // 5 MiB

func main() {
	// Initialize log collector for admin panel
	logCollector := adminpanel.NewLogCollector(500)

	// Setup multi-handler: stdout + collector
	stdoutHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	collectorHandler := adminpanel.NewSlogHandler(logCollector)
	multiHandler := adminpanel.NewMultiHandler(stdoutHandler, collectorHandler)
	slog.SetDefault(slog.New(multiHandler))

	// Load configuration from environment
	cfg, err := model.LoadConfigFromEnv()
	if err != nil {
		slog.Error("Configuration error", "error", err)
		os.Exit(1)
	}

	// Initialize database
	database, err := db.New(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := database.Close(); err != nil {
			slog.Warn("Failed to close database", "error", err)
		}
	}()

	// Apply schema and migrations
	if err := database.InitDatabase(); err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}

	// Create bot
	b, err := bot.New(cfg, database)
	if err != nil {
		slog.Error("Failed to create bot", "error", err)
		os.Exit(1)
	}

	// Register commands with Telegram
	ctx := context.Background()
	if err := bot.SetCommands(ctx, b); err != nil {
		slog.Warn("Failed to set bot commands", "error", err)
	}

	// Setup HTTP server
	mux := http.NewServeMux()

	// Telegram webhook endpoint
	mux.HandleFunc("/tghook", b.WebhookHandler())

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprint(w, "OK"); err != nil {
			slog.Warn("Failed to write health response", "error", err)
		}
	})

	// Admin panel — ServeMux auto-redirects /admin → /admin/
	adminHandler := adminpanel.NewServer(database, cfg.BotToken, cfg.BotUsername, logCollector)
	mux.Handle("/admin/", http.StripPrefix("/admin", adminHandler))

	// GitHub webhook endpoint
	mux.HandleFunc("/github-webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if cfg.GitHubWebhookSecret == "" {
			http.Error(w, "Webhook not configured", http.StatusServiceUnavailable)
			return
		}

		if cfg.ReleaseChannelID == 0 {
			http.Error(w, "Release channel not configured", http.StatusServiceUnavailable)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGitHubWebhookBodyBytes))
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		sig := r.Header.Get("X-Hub-Signature-256")
		channelID := fmt.Sprintf("%d", cfg.ReleaseChannelID)

		if err := github.HandleGitHubWebhook(r.Context(), body, sig, cfg.BotToken, channelID, cfg.GitHubWebhookSecret); err != nil {
			slog.Error("GitHub webhook error", "error", err)
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprint(w, "OK"); err != nil {
			slog.Warn("Failed to write webhook response", "error", err)
		}
	})

	// Set webhook endpoint (for initial setup)
	mux.HandleFunc("/set-webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		token := r.URL.Query().Get("token")
		if cfg.GitHubWebhookSecret == "" || token != cfg.GitHubWebhookSecret {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		scheme := "https"
		if r.Header.Get("X-Forwarded-Proto") != "" {
			scheme = r.Header.Get("X-Forwarded-Proto")
		}
		host := r.Host
		webhookURL := fmt.Sprintf("%s://%s/tghook", scheme, host)

		_, err := b.SetWebhook(ctx, &tgbot.SetWebhookParams{
			URL:            webhookURL,
			AllowedUpdates: []string{"message", "chat_member", "callback_query"},
		})
		if err != nil {
			slog.Error("Failed to set webhook", "error", err)
			http.Error(w, "Failed to set webhook", http.StatusInternalServerError)
			return
		}

		if _, err := fmt.Fprintf(w, "Webhook set to %s", webhookURL); err != nil {
			slog.Warn("Failed to write set-webhook response", "error", err)
		}
	})

	// Start bot workers
	go b.StartWebhook(ctx)

	// Configure HTTP server
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		slog.Info("Shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("Server shutdown error", "error", err)
		}
	}()

	slog.Info("Neptune starting", "addr", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
	slog.Info("Neptune stopped")
}
