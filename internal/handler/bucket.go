package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/domain/bucket"
)

// BucketService is the interface the BucketHandler depends on.
type BucketService interface {
	Create(ctx context.Context, req bucket.CreateRequest) (bucket.CreateResult, error)
	GetMeta(ctx context.Context, bucketSlug, passwordAttempt string) (bucket.GetResult, error)
	GetFile(ctx context.Context, bucketSlug, storageKey, passwordAttempt string) (bucket.BucketFile, io.ReadCloser, error)
	StreamZIP(ctx context.Context, bucketSlug, passwordAttempt string, w http.ResponseWriter) error
	Delete(ctx context.Context, bucketSlug, deleteSecret string) error
}

// BucketHandler handles bucket upload, view, file download, ZIP download, and deletion.
type BucketHandler struct {
	svc            BucketService
	maxUploadBytes int64
}

// NewBucketHandler constructs a BucketHandler.
func NewBucketHandler(svc BucketService, maxUploadBytes int64) *BucketHandler {
	return &BucketHandler{svc: svc, maxUploadBytes: maxUploadBytes}
}

// bucketUploadResponse is the JSON shape returned after a successful bucket upload.
type bucketUploadResponse struct {
	URL       string           `json:"url"`
	DeleteURL string           `json:"delete_url"`
	ExpiresAt *string          `json:"expires_at"` // RFC3339 or null
	FileCount int              `json:"file_count"`
	Files     []bucket.FileInfo `json:"files"`
}

// Upload handles POST /api/upload?type=bucket (multipart form with multiple file fields).
func (h *BucketHandler) Upload(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)

	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes)

	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("upload exceeds maximum size of %d bytes", h.maxUploadBytes))
			return
		}
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	defer r.MultipartForm.RemoveAll()

	fileHeaders := r.MultipartForm.File["file"]
	if len(fileHeaders) == 0 {
		writeError(w, http.StatusBadRequest, "missing 'file' field(s) in form")
		return
	}

	var fileInputs []bucket.FileInput
	var openedFiles []io.Closer

	for _, fhdr := range fileHeaders {
		fh, err := fhdr.Open()
		if err != nil {
			for _, c := range openedFiles {
				c.Close()
			}
			writeError(w, http.StatusInternalServerError, "failed to open uploaded file")
			return
		}
		openedFiles = append(openedFiles, fh)

		filename := fhdr.Filename
		if filename == "" {
			filename = "upload"
		}

		// Detect MIME type from first 512 bytes, then re-combine for streaming.
		peek := make([]byte, 512)
		n, _ := fh.Read(peek)
		peek = peek[:n]
		mimeType := http.DetectContentType(peek)

		// Force application/octet-stream for dangerous types.
		switch mimeType {
		case "text/html", "text/xml", "image/svg+xml", "application/xhtml+xml":
			mimeType = "application/octet-stream"
		}

		content := io.MultiReader(bytes.NewReader(peek), fh)

		fileInputs = append(fileInputs, bucket.FileInput{
			Filename: filename,
			MimeType: mimeType,
			Size:     fhdr.Size,
			Content:  content,
		})
	}

	expiry := r.FormValue("expiry")
	if expiry == "" {
		expiry = "1d"
	}
	password := r.FormValue("password")
	oneUse := r.FormValue("one_use") == "1"

	result, err := h.svc.Create(r.Context(), bucket.CreateRequest{
		Files:    fileInputs,
		Expiry:   expiry,
		Password: password,
		OneUse:   oneUse,
	})

	// Close all opened file handles after Create returns (it reads them synchronously).
	for _, c := range openedFiles {
		c.Close()
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "bucket upload failed")
		return
	}

	resp := bucketUploadResponse{
		URL:       result.URL,
		DeleteURL: result.DeleteURL,
		FileCount: result.FileCount,
		Files:     result.Files,
	}
	if result.ExpiresAt != nil {
		s := result.ExpiresAt.UTC().Format(time.RFC3339)
		resp.ExpiresAt = &s
	}

	writeJSON(w, http.StatusCreated, resp)
}

// View handles GET /b/{slug} — renders an HTML page listing bucket files.
func (h *BucketHandler) View(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)

	slug := r.PathValue("slug")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "missing slug")
		return
	}

	password := r.Header.Get("X-Password")
	if password == "" {
		password = r.URL.Query().Get("password")
	}

	result, err := h.svc.GetMeta(r.Context(), slug, password)
	if err != nil {
		switch {
		case errors.Is(err, bucket.ErrWrongPassword):
			acceptJSON := r.Header.Get("Accept") == "application/json"
			if acceptJSON || r.Header.Get("X-Password") != "" {
				writeError(w, http.StatusUnauthorized, "wrong password")
			} else {
				servePasswordForm(w, "b/"+slug)
			}
		case errors.Is(err, bucket.ErrExpired):
			writeError(w, http.StatusGone, "bucket has expired")
		case errors.Is(err, bucket.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'")

	b := result.B

	// Build expiry info string.
	expiryInfo := "never expires"
	if result.ExpiresAt != nil {
		expiryInfo = "expires " + result.ExpiresAt.UTC().Format("2006-01-02 15:04 UTC")
	}

	// Build per-file rows.
	passwordQuery := ""
	if password != "" {
		passwordQuery = "?password=" + password
	}

	zipURL := fmt.Sprintf("/b/%s/zip%s", slug, passwordQuery)

	fileRows := ""
	for _, bf := range b.Files {
		fileURL := fmt.Sprintf("/b/%s/file/%s%s", slug, bf.StorageKey, passwordQuery)
		fileRows += fmt.Sprintf(
			`<tr><td><a href="%s" download="%s">%s</a></td><td>%s</td><td><a href="%s" download="%s" class="btn-dl">Download</a></td></tr>`,
			fileURL,
			htmlEscape(bf.Filename),
			htmlEscape(bf.Filename),
			humanSize(bf.Size),
			fileURL,
			htmlEscape(bf.Filename),
		)
	}

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>%s — pbin</title>
<style>
body{font-family:sans-serif;max-width:700px;margin:2rem auto;padding:1rem;color:#222}
h1{font-size:1.4rem;margin-bottom:.5rem}
.meta{font-size:.85rem;color:#666;margin-bottom:1.5rem}
table{width:100%%;border-collapse:collapse;margin-bottom:1.5rem}
th{text-align:left;font-size:.85rem;color:#555;border-bottom:2px solid #ddd;padding:.4rem .5rem}
td{padding:.5rem .5rem;border-bottom:1px solid #eee;font-size:.9rem;word-break:break-all}
td:last-child{white-space:nowrap;text-align:right}
a{color:#0066cc;text-decoration:none}
a:hover{text-decoration:underline}
.btn-dl{font-size:.8rem;background:#0066cc;color:#fff;padding:.2rem .6rem;border-radius:3px}
.btn-dl:hover{background:#0052a3;text-decoration:none}
.btn-zip{display:inline-block;background:#28a745;color:#fff;padding:.6rem 1.2rem;border-radius:4px;font-size:1rem;margin-top:.5rem}
.btn-zip:hover{background:#1e7e34;text-decoration:none}
</style></head>
<body>
<h1>%s</h1>
<div class="meta">%d file(s) &mdash; %s</div>
<table>
<thead><tr><th>Filename</th><th>Size</th><th></th></tr></thead>
<tbody>%s</tbody>
</table>
<a href="%s" class="btn-zip">Download all as ZIP</a>
</body></html>`,
		slug,
		slug,
		len(b.Files),
		expiryInfo,
		fileRows,
		zipURL,
	)
}

// DownloadFile handles GET /b/{slug}/file/{storageKey} — serves an individual bucket file.
func (h *BucketHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)

	slug := r.PathValue("slug")
	storageKey := r.PathValue("storageKey")

	if slug == "" || storageKey == "" {
		writeError(w, http.StatusBadRequest, "missing slug or storageKey")
		return
	}

	password := r.Header.Get("X-Password")
	if password == "" {
		password = r.URL.Query().Get("password")
	}

	bf, rc, err := h.svc.GetFile(r.Context(), slug, storageKey, password)
	if err != nil {
		switch {
		case errors.Is(err, bucket.ErrWrongPassword):
			writeError(w, http.StatusUnauthorized, "wrong password")
		case errors.Is(err, bucket.ErrExpired):
			writeError(w, http.StatusGone, "bucket has expired")
		case errors.Is(err, bucket.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	defer rc.Close()

	mimeType := bf.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, bf.Filename))

	io.Copy(w, rc)
}

// DownloadZIP handles GET /b/{slug}/zip — streams all bucket files as a ZIP.
func (h *BucketHandler) DownloadZIP(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)

	slug := r.PathValue("slug")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "missing slug")
		return
	}

	password := r.Header.Get("X-Password")
	if password == "" {
		password = r.URL.Query().Get("password")
	}

	// StreamZIP writes headers and body directly to w; must check errors before calling.
	// The service handles all auth/expiry checks before writing any bytes.
	if err := h.svc.StreamZIP(r.Context(), slug, password, w); err != nil {
		switch {
		case errors.Is(err, bucket.ErrWrongPassword):
			acceptJSON := r.Header.Get("Accept") == "application/json"
			if acceptJSON || r.Header.Get("X-Password") != "" {
				writeError(w, http.StatusUnauthorized, "wrong password")
			} else {
				servePasswordForm(w, "b/"+slug)
			}
		case errors.Is(err, bucket.ErrExpired):
			writeError(w, http.StatusGone, "bucket has expired")
		case errors.Is(err, bucket.ErrAlreadyConsumed):
			writeError(w, http.StatusGone, "bucket has already been downloaded")
		case errors.Is(err, bucket.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
}

// DeleteBucket handles GET /b/delete/{slug}/{secret} — removes a bucket and all its files.
func (h *BucketHandler) DeleteBucket(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)

	slug := r.PathValue("slug")
	secret := r.PathValue("secret")

	if slug == "" || secret == "" {
		writeError(w, http.StatusBadRequest, "missing slug or secret")
		return
	}

	if err := h.svc.Delete(r.Context(), slug, secret); err != nil {
		switch {
		case errors.Is(err, bucket.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		case errors.Is(err, bucket.ErrBadDeleteSecret):
			writeError(w, http.StatusForbidden, "invalid delete token")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// humanSize returns a human-readable file size string.
func humanSize(n int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(GB))
	case n >= MB:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(MB))
	case n >= KB:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(KB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// htmlEscape escapes special HTML characters in s.
func htmlEscape(s string) string {
	var buf bytes.Buffer
	for _, r := range s {
		switch r {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '"':
			buf.WriteString("&#34;")
		case '\'':
			buf.WriteString("&#39;")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
