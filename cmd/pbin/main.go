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
	"strconv"
	"syscall"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/config"
	"github.com/ahmethakanbesel/pbin/internal/domain/bucket"
	"github.com/ahmethakanbesel/pbin/internal/domain/file"
	"github.com/ahmethakanbesel/pbin/internal/domain/paste"
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
	slog.Info("file store ready", "path", cfg.Storage.Path)

	// Build base URL from config (used in shareable URLs returned to uploaders).
	baseURL := "http://" + cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port)

	fileRepo := storage.NewFileRepo(db)
	fileSvc := file.NewService(fileRepo, fs, baseURL)
	fileHandler := handler.NewFileHandler(fileSvc, cfg.Upload.MaxBytes)

	bucketRepo := storage.NewBucketRepo(db)
	bucketSvc := bucket.NewService(bucketRepo, fs, baseURL)
	bucketHandler := handler.NewBucketHandler(bucketSvc, cfg.Upload.MaxBytes)

	pasteRepo := storage.NewPasteRepo(db)
	pasteSvc := paste.NewService(pasteRepo, baseURL)
	pasteHandler := handler.NewPasteHandler(pasteSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)

	// Upload: dispatch on ?type=bucket
	mux.HandleFunc("POST /api/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") == "bucket" {
			bucketHandler.Upload(w, r)
		} else {
			fileHandler.Upload(w, r)
		}
	})

	// Paste creation
	mux.HandleFunc("POST /api/paste", pasteHandler.Create)

	// More-specific patterns before the wildcard (Go 1.22 requires this)
	mux.HandleFunc("GET /{slug}/info", fileHandler.Info)
	mux.HandleFunc("GET /{slug}/zip", bucketHandler.DownloadZIP)
	mux.HandleFunc("GET /{slug}/file/{storageKey}", bucketHandler.DownloadFile)
	mux.HandleFunc("GET /raw/{slug}", pasteHandler.Raw)

	// Universal slug dispatch: files, buckets, pastes
	mux.HandleFunc("GET /{slug}", func(w http.ResponseWriter, r *http.Request) {
		sl := r.PathValue("slug")

		// Try file — route to fileHandler for all "file exists" signals:
		// err == nil (not expired, any password state), ErrWrongPassword (never from GetMeta but
		// included per spec), ErrExpired. Only skip if ErrNotFound.
		if _, err := fileSvc.GetMeta(r.Context(), sl); err == nil ||
			errors.Is(err, file.ErrWrongPassword) || errors.Is(err, file.ErrExpired) {
			fileHandler.Serve(w, r)
			return
		}
		// Try bucket — route to bucketHandler for all "bucket exists" signals.
		if _, err := bucketSvc.GetMeta(r.Context(), sl, ""); err == nil ||
			errors.Is(err, bucket.ErrWrongPassword) || errors.Is(err, bucket.ErrExpired) {
			bucketHandler.View(w, r)
			return
		}
		// Fall through to paste handler — it handles ErrNotFound with a 404.
		pasteHandler.View(w, r)
	})

	// Universal delete dispatch
	mux.HandleFunc("GET /delete/{slug}/{secret}", func(w http.ResponseWriter, r *http.Request) {
		sl := r.PathValue("slug")

		// Identify entity type by slug existence only (not by secret).
		// Use GetMeta with empty password — only care whether ErrNotFound comes back.
		if _, err := fileSvc.GetMeta(r.Context(), sl); !errors.Is(err, file.ErrNotFound) {
			fileHandler.Delete(w, r)
			return
		}
		if _, err := bucketSvc.GetMeta(r.Context(), sl, ""); !errors.Is(err, bucket.ErrNotFound) {
			bucketHandler.DeleteBucket(w, r)
			return
		}
		pasteHandler.Delete(w, r) // will return 404 if not found
	})

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
