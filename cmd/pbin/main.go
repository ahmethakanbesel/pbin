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
	"strings"
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

	// Fixed routes that do not conflict with the GET / catch-all.
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("POST /api/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") == "bucket" {
			bucketHandler.Upload(w, r)
		} else {
			fileHandler.Upload(w, r)
		}
	})
	mux.HandleFunc("POST /api/paste", pasteHandler.Create)

	// GET / catch-all — handles all GET requests that are not /health.
	// Manual routing is used here because Go 1.22's ServeMux panics when patterns overlap
	// without one being strictly more specific (e.g. /{slug}/info vs /b/{slug} both match /b/info).
	// By consolidating all GET routing here we avoid any pattern conflicts while preserving
	// the full URL structure that bucket and paste services generate.
	//
	// URL structure:
	//   GET /{slug}                        → file or paste view (slug dispatcher)
	//   GET /{slug}/info                   → file info (embed/preview page)
	//   GET /{slug}/raw                    → paste raw text
	//   GET /b/{slug}                      → bucket view (file listing)
	//   GET /b/{slug}/zip                  → bucket ZIP download
	//   GET /b/{slug}/file/{storageKey}    → individual bucket file download
	//   GET /b/delete/{slug}/{secret}      → bucket delete
	//   GET /delete/{slug}/{secret}        → file or paste delete (slug dispatcher)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// Split path into non-empty segments.
		path := strings.Trim(r.URL.Path, "/")
		var parts []string
		if path != "" {
			parts = strings.Split(path, "/")
		}

		n := len(parts)

		switch {
		case n == 0:
			// GET / — no content at root
			http.NotFound(w, r)

		case n == 1 && parts[0] == "health":
			// Should be caught by the exact GET /health pattern above, but guard here.
			handler.Health(w, r)

		// ── File info ─────────────────────────────────────────────────────────
		case n == 2 && parts[1] == "info":
			// GET /{slug}/info
			r.SetPathValue("slug", parts[0])
			fileHandler.Info(w, r)

		// ── Paste raw ─────────────────────────────────────────────────────────
		case n == 2 && parts[1] == "raw":
			// GET /{slug}/raw
			r.SetPathValue("slug", parts[0])
			pasteHandler.Raw(w, r)

		// ── Bucket sub-routes ─────────────────────────────────────────────────
		case n == 2 && parts[0] == "b" && parts[1] != "":
			// GET /b/{slug} — bucket view
			r.SetPathValue("slug", parts[1])
			bucketHandler.View(w, r)

		case n == 3 && parts[0] == "b" && parts[2] == "zip":
			// GET /b/{slug}/zip
			r.SetPathValue("slug", parts[1])
			bucketHandler.DownloadZIP(w, r)

		case n == 4 && parts[0] == "b" && parts[2] == "file":
			// GET /b/{slug}/file/{storageKey}
			r.SetPathValue("slug", parts[1])
			r.SetPathValue("storageKey", parts[3])
			bucketHandler.DownloadFile(w, r)

		case n == 4 && parts[0] == "b" && parts[1] == "delete":
			// GET /b/delete/{slug}/{secret}
			r.SetPathValue("slug", parts[2])
			r.SetPathValue("secret", parts[3])
			bucketHandler.DeleteBucket(w, r)

		// ── Delete dispatcher (files and pastes) ──────────────────────────────
		case n == 3 && parts[0] == "delete":
			// GET /delete/{slug}/{secret}
			sl := parts[1]
			r.SetPathValue("slug", sl)
			r.SetPathValue("secret", parts[2])

			// Identify entity type by slug existence only (not by secret).
			// GetMeta does not check password; only ErrNotFound means the slug is absent.
			if _, err := fileSvc.GetMeta(r.Context(), sl); !errors.Is(err, file.ErrNotFound) {
				fileHandler.Delete(w, r)
				return
			}
			pasteHandler.Delete(w, r) // returns 404 if not found

		// ── Universal slug view (files and pastes) ────────────────────────────
		case n == 1:
			// GET /{slug}
			sl := parts[0]
			r.SetPathValue("slug", sl)

			// Try file — route to fileHandler for all "file exists" signals.
			// GetMeta does not check password, so password-protected files return nil.
			// ErrExpired means the file exists (just expired). Only skip on ErrNotFound.
			if _, err := fileSvc.GetMeta(r.Context(), sl); err == nil ||
				errors.Is(err, file.ErrWrongPassword) || errors.Is(err, file.ErrExpired) {
				fileHandler.Serve(w, r)
				return
			}
			// Fall through to paste — pasteHandler.View returns 404 if slug not found.
			pasteHandler.View(w, r)

		default:
			http.NotFound(w, r)
		}
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
