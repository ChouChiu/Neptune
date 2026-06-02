package adminpanel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// LogEntry represents a single log entry.
type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Attrs   string `json:"attrs,omitempty"`
}

// LogCollector collects log entries and broadcasts them to subscribers.
type LogCollector struct {
	mu          sync.RWMutex
	subscribers map[chan LogEntry]struct{}
	buffer      []LogEntry
	maxBuffer   int
}

// NewLogCollector creates a new LogCollector.
func NewLogCollector(maxBuffer int) *LogCollector {
	return &LogCollector{
		subscribers: make(map[chan LogEntry]struct{}),
		buffer:      make([]LogEntry, 0, maxBuffer),
		maxBuffer:   maxBuffer,
	}
}

// Subscribe creates a new channel to receive log entries.
func (lc *LogCollector) Subscribe() chan LogEntry {
	ch := make(chan LogEntry, 100)
	lc.mu.Lock()
	lc.subscribers[ch] = struct{}{}
	lc.mu.Unlock()

	// Send buffer history to new subscriber
	lc.mu.RLock()
	for _, entry := range lc.buffer {
		select {
		case ch <- entry:
		default:
		}
	}
	lc.mu.RUnlock()

	return ch
}

// Unsubscribe removes a subscriber channel.
func (lc *LogCollector) Unsubscribe(ch chan LogEntry) {
	lc.mu.Lock()
	delete(lc.subscribers, ch)
	lc.mu.Unlock()
	close(ch)
}

// Add adds a log entry and broadcasts it to all subscribers.
func (lc *LogCollector) Add(entry LogEntry) {
	lc.mu.Lock()
	lc.buffer = append(lc.buffer, entry)
	if len(lc.buffer) > lc.maxBuffer {
		lc.buffer = lc.buffer[1:]
	}
	for ch := range lc.subscribers {
		select {
		case ch <- entry:
		default:
			// Drop if subscriber is slow
		}
	}
	lc.mu.Unlock()
}

// slogHandler implements slog.Handler to collect logs.
type slogHandler struct {
	collector *LogCollector
	attrs     []slog.Attr
}

// NewSlogHandler creates a new slog.Handler that collects logs.
func NewSlogHandler(collector *LogCollector) slog.Handler {
	return &slogHandler{collector: collector}
}

func (h *slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

func (h *slogHandler) Handle(_ context.Context, r slog.Record) error {
	level := r.Level.String()
	switch r.Level {
	case slog.LevelDebug:
		level = "DEBUG"
	case slog.LevelInfo:
		level = "INFO"
	case slog.LevelWarn:
		level = "WARN"
	case slog.LevelError:
		level = "ERROR"
	}

	attrs := ""
	r.Attrs(func(a slog.Attr) bool {
		if attrs != "" {
			attrs += " "
		}
		attrs += fmt.Sprintf("%s=%v", a.Key, a.Value.Any())
		return true
	})

	handlerAttrs := ""
	for _, a := range h.attrs {
		if handlerAttrs != "" {
			handlerAttrs += " "
		}
		handlerAttrs += fmt.Sprintf("%s=%v", a.Key, a.Value.Any())
	}
	if handlerAttrs != "" {
		if attrs != "" {
			attrs = handlerAttrs + " " + attrs
		} else {
			attrs = handlerAttrs
		}
	}

	entry := LogEntry{
		Time:    r.Time.Format(time.TimeOnly),
		Level:   level,
		Message: r.Message,
		Attrs:   attrs,
	}
	h.collector.Add(entry)
	return nil
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &slogHandler{
		collector: h.collector,
		attrs:     newAttrs,
	}
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	return h
}

// MultiHandler fans out log records to multiple slog.Handlers.
type MultiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler creates a slog.Handler that writes to multiple handlers.
func NewMultiHandler(handlers ...slog.Handler) slog.Handler {
	return &MultiHandler{handlers: handlers}
}

func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			_ = h.Handle(ctx, r)
		}
	}
	return nil
}

func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: handlers}
}

func (m *MultiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: handlers}
}
