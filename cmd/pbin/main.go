package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/config"
	"github.com/ahmethakanbesel/pbin/internal/filestore"
	"github.com/ahmethakanbesel/pbin/internal/handler"
	"github.com/ahmethakanbesel/pbin/internal/storage"
)

func main() {
	configPath := flag.String("config", "pbin.toml", "path to TOML config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Parse(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Ensure storage directory exists
	for _, dir := range []string{cfg.Storage.Path} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Error("failed to create directory", "path", dir, "error", err)
			os.Exit(1)
		}
	}

	// Ensure DB parent directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.Database.Path), 0755); err != nil {
		slog.Error("failed to create db directory", "error", err)
		os.Exit(1)
	}

	db, err := storage.Open(cfg.Database.Path)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("database ready", "path", cfg.Database.Path)

	fs, err := filestore.NewLocal(cfg.Storage.Path)
	if err != nil {
		slog.Error("failed to initialize file store", "error", err)
		os.Exit(1)
	}
	_ = fs // will be used in Phase 2
	slog.Info("file store ready", "path", cfg.Storage.Path)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown on SIGTERM / SIGINT
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		slog.Info("server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}
