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

	tgbot "github.com/go-telegram/bot"
	"github.com/kazumi-group/neptune/internal/adminpanel"
	"github.com/kazumi-group/neptune/internal/bot"
	"github.com/kazumi-group/neptune/internal/db"
	"github.com/kazumi-group/neptune/internal/github"
	"github.com/kazumi-group/neptune/internal/model"
)

func main() {
	// Load configuration from environment
	cfg := &model.Config{
		BotToken:            os.Getenv("BOT_TOKEN"),
		BotUsername:         os.Getenv("BOT_USERNAME"),
		HermesAPIURL:        os.Getenv("HERMES_API_URL"),
		HermesAPIKey:        os.Getenv("HERMES_API_KEY"),
		ReuseCaptcha:        os.Getenv("REUSE_CAPTCHA") == "true",
		GitHubWebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
	}

	if cfg.HermesAPIURL == "" {
		cfg.HermesAPIURL = "http://127.0.0.1:8642/v1"
	}

	if cfg.BotToken == "" {
		slog.Error("BOT_TOKEN environment variable is required")
		os.Exit(1)
	}

	// Parse RELEASE_CHANNEL_ID
	if id := os.Getenv("RELEASE_CHANNEL_ID"); id != "" {
		fmt.Sscanf(id, "%d", &cfg.ReleaseChannelID)
	}

	// Initialize database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/neptune.db"
	}
	database, err := db.New(dbPath)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Apply schema
	if err := database.ApplySchema(); err != nil {
		slog.Error("Failed to apply schema", "error", err)
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
	mux.HandleFunc("/webhook", b.WebhookHandler())

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	// Admin panel — ServeMux auto-redirects /admin → /admin/
	adminHandler := adminpanel.NewServer(database, cfg.BotToken, cfg.BotUsername)
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

		body, err := io.ReadAll(r.Body)
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
		fmt.Fprint(w, "OK")
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
		webhookURL := fmt.Sprintf("%s://%s/webhook", scheme, host)

		_, err := b.SetWebhook(ctx, &tgbot.SetWebhookParams{
			URL:            webhookURL,
			AllowedUpdates: []string{"message", "chat_member", "callback_query"},
		})
		if err != nil {
			slog.Error("Failed to set webhook", "error", err)
			http.Error(w, "Failed to set webhook", http.StatusInternalServerError)
			return
		}

		// Also set commands
		if err := bot.SetCommands(ctx, b); err != nil {
			slog.Warn("Failed to set commands", "error", err)
		}

		fmt.Fprintf(w, "Webhook set to %s", webhookURL)
	})

	// Start bot workers
	go b.StartWebhook(ctx)

	// Configure HTTP server
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:         addr,
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

	slog.Info("Neptune starting", "addr", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
	slog.Info("Neptune stopped")
}
