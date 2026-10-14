// Package handler provides HTTP handlers for the pbin service.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/domain/file"
)

const (
	multipartMemory = 32 << 20 // 32 MB max in-memory multipart buffer
)

// FileService is the interface the handler depends on (allows mock injection in tests).
type FileService interface {
	Upload(ctx context.Context, req file.UploadRequest) (file.UploadResult, error)
	Get(ctx context.Context, shareSlug, passwordAttempt string) (file.GetResult, error)
	GetMeta(ctx context.Context, shareSlug string) (file.File, error)
	Delete(ctx context.Context, shareSlug, deleteSecret string) error
}

// FileHandler handles file upload, download, share info, and deletion.
type FileHandler struct {
	svc            FileService
	maxUploadBytes int64
}

// NewFileHandler constructs a FileHandler.
func NewFileHandler(svc FileService, maxUploadBytes int64) *FileHandler {
	return &FileHandler{svc: svc, maxUploadBytes: maxUploadBytes}
}

// securityHeaders sets security headers required on all responses.
func securityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
}

// writeJSON encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// uploadResponse is the JSON shape returned after a successful upload.
type uploadResponse struct {
	URL         string  `json:"url"`          // share page (/{slug}/info)
	DownloadURL string  `json:"download_url"` // raw file (/{slug}), used for embeds
	DeleteURL   string  `json:"delete_url"`
	ExpiresAt   *string `json:"expires_at"` // RFC3339 or null
	IsImage     bool    `json:"is_image"`
}

// Upload handles POST /api/upload (multipart form).
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)

	// Enforce upload size before any parsing.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes)

	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file exceeds maximum size of %d bytes", h.maxUploadBytes))
			return
		}
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	defer r.MultipartForm.RemoveAll()

	// Call r.FormFile once; capture both the file handle and header.
	fh, fhdr, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'file' field in form")
		return
	}
	defer fh.Close()

	filename := fhdr.Filename
	if filename == "" {
		filename = "upload"
	}
	size := fhdr.Size

	// Detect MIME type from first 512 bytes, then re-combine for streaming.
	peek := make([]byte, 512)
	n, _ := fh.Read(peek)
	peek = peek[:n]
	mimeType := http.DetectContentType(peek)

	// Force application/octet-stream for dangerous types.
	// SVG comes as text/xml or image/svg+xml — both are unsafe for inline serving.
	switch mimeType {
	case "text/html", "text/xml", "image/svg+xml", "application/xhtml+xml":
		mimeType = "application/octet-stream"
	}

	// If the detected MIME type claims to be an image, verify magic bytes.
	if file.IsImage(mimeType) && !validateImageMagic(mimeType, peek) {
		mimeType = "application/octet-stream"
	}

	// Re-combine peeked bytes with remaining file content.
	content := io.MultiReader(bytes.NewReader(peek), fh)

	expiry := r.FormValue("expiry")
	if expiry == "" {
		expiry = "1d"
	}
	password := r.FormValue("password")
	oneUse := r.FormValue("one_use") == "1"

	result, err := h.svc.Upload(r.Context(), file.UploadRequest{
		Filename: filename,
		MimeType: mimeType,
		Size:     size,
		Expiry:   expiry,
		Password: password,
		OneUse:   oneUse,
		Content:  content,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload failed")
		return
	}

	resp := uploadResponse{
		URL:         absURL(r, result.URL),
		DownloadURL: absURL(r, "/"+result.Slug),
		DeleteURL:   absURL(r, result.DeleteURL),
		IsImage:     result.IsImage,
	}
	if result.ExpiresAt != nil {
		s := result.ExpiresAt.UTC().Format(time.RFC3339)
		resp.ExpiresAt = &s
	}

	writeJSON(w, http.StatusCreated, resp)
}

// Serve handles GET /{slug} — file download with expiry/one-use/password enforcement.
func (h *FileHandler) Serve(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)

	slug := r.PathValue("slug")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "missing slug")
		return
	}

	password := r.Header.Get("X-Password")
	// Also accept ?password= query param for browser form submission.
	if password == "" {
		password = r.URL.Query().Get("password")
	}

	result, err := h.svc.Get(r.Context(), slug, password)
	if err != nil {
		switch {
		case errors.Is(err, file.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		case errors.Is(err, file.ErrExpired):
			writeError(w, http.StatusGone, "file has expired")
		case errors.Is(err, file.ErrAlreadyConsumed):
			writeError(w, http.StatusGone, "file has already been downloaded")
		case errors.Is(err, file.ErrWrongPassword):
			// Show password form for browser requests; 401 JSON for API requests.
			acceptJSON := r.Header.Get("Accept") == "application/json"
			if acceptJSON || r.Header.Get("X-Password") != "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "wrong password"})
			} else {
				servePasswordForm(w, slug)
			}
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	defer result.Content.Close()

	// Set Content-Security-Policy for share pages.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")

	filename := result.F.Filename
	if filename == "" {
		filename = slug
	}

	if result.IsImage {
		w.Header().Set("Content-Type", result.F.MimeType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename=%q`, filename))
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	}

	io.Copy(w, result.Content)
}

// servePasswordForm writes a styled HTML password prompt consistent with the app design.
func servePasswordForm(w http.ResponseWriter, slug string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline' https://cdn.jsdelivr.net; form-action 'self'")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Password Required — pbin</title>
<link rel="stylesheet" href="%s">
%s
<style>
.pw-card{max-width:420px;margin:3rem auto;padding:2rem;background:var(--pbin-surface);border:1px solid var(--pbin-surface-border);border-radius:var(--pbin-radius-lg)}
.pw-card h2{margin-top:0;font-size:1.25rem}
.pw-card p{color:var(--pbin-muted);font-size:.9rem}
.pw-icon{display:block;margin:0 auto 1rem;text-align:center;color:var(--pbin-muted)}
.pw-icon svg{width:40px;height:40px}
</style>
</head>
<body>
%s
<main class="container">
<div class="pw-card">
  <div class="pw-icon"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z"/></svg></div>
  <h2>Password Required</h2>
  <p>This content is password protected. Enter the password to continue.</p>
  <form method="GET" action="/%s">
    <label for="pw">Password
      <input id="pw" type="password" name="password" required autofocus placeholder="Enter password">
    </label>
    <button type="submit">Continue</button>
  </form>
</div>
</main>
%s
</body>
</html>`, picoCSS, customCSS, viewNavBarHTML(), slug, footerHTML)
}

// deleteResponse is the JSON shape returned after a successful deletion.
type deleteResponse struct {
	Deleted bool `json:"deleted"`
}

// Delete handles GET /delete/{slug}/{secret}.
func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)

	slug := r.PathValue("slug")
	secret := r.PathValue("secret")

	if slug == "" || secret == "" {
		writeError(w, http.StatusBadRequest, "missing slug or secret")
		return
	}

	if err := h.svc.Delete(r.Context(), slug, secret); err != nil {
		switch {
		case errors.Is(err, file.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		case errors.Is(err, file.ErrBadDeleteSecret):
			writeError(w, http.StatusForbidden, "invalid delete token")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, deleteResponse{Deleted: true})
}

// imageMagicBytes maps validated MIME types to their magic byte signatures.
var imageMagicBytes = map[string][][]byte{
	"image/png":  {{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}},
	"image/jpeg": {{0xFF, 0xD8, 0xFF}},
	"image/gif":  {[]byte("GIF87a"), []byte("GIF89a")},
	"image/bmp":  {{0x42, 0x4D}},
	// WebP checked separately below (RIFF....WEBP pattern)
}

// validateImageMagic reports whether the first bytes of peek match known image signatures
// for the given MIME type. For WebP: bytes 0-3 must be "RIFF" and bytes 8-11 must be "WEBP".
func validateImageMagic(mimeType string, peek []byte) bool {
	if len(peek) < 4 {
		return false
	}
	switch mimeType {
	case "image/webp":
		return len(peek) >= 12 &&
			string(peek[0:4]) == "RIFF" &&
			string(peek[8:12]) == "WEBP"
	default:
		sigs, ok := imageMagicBytes[mimeType]
		if !ok {
			return false
		}
		for _, sig := range sigs {
			if len(peek) >= len(sig) && bytes.Equal(peek[:len(sig)], sig) {
				return true
			}
		}
	}
	return false
}

// Info handles GET /{slug}/info — renders an HTML share page for any file: metadata, a
// download button and copyable links, plus a preview and embed codes for images.
func (h *FileHandler) Info(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)

	slug := r.PathValue("slug")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "missing slug")
		return
	}

	f, err := h.svc.GetMeta(r.Context(), slug)
	if err != nil {
		switch {
		case errors.Is(err, file.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		case errors.Is(err, file.ErrExpired):
			writeError(w, http.StatusGone, "file has expired")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	infoFilename := f.Filename
	if infoFilename == "" {
		infoFilename = slug
	}
	isImage := file.IsImage(f.MimeType)
	protected := f.PasswordHash != ""
	fileURL := baseURL(r) + "/" + slug

	// Inline preview fetches /{slug}, which would consume a one-use file
	// and cannot pass a password, so only preview unrestricted images.
	preview := ""
	if isImage && !protected && !f.OneUse {
		preview = fmt.Sprintf(`<img class="preview-img" src="/%s" alt="%s">`, slug, html.EscapeString(infoFilename))
	}

	mimeType := f.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	meta := []string{humanSize(f.Size), html.EscapeString(mimeType)}
	if f.ExpiresAt != nil {
		meta = append(meta, "Expires "+f.ExpiresAt.UTC().Format("2006-01-02 15:04 UTC"))
	} else {
		meta = append(meta, "Never expires")
	}
	if protected {
		meta = append(meta, "Password protected")
	}
	if f.OneUse {
		meta = append(meta, "Can be downloaded only once")
	}
	metaHTML := `<span>` + strings.Join(meta, `</span><span class="meta-sep"></span><span>`) + `</span>`

	embedRow := func(label, value string) string {
		v := html.EscapeString(value)
		return fmt.Sprintf(`<div class="embed-group"><div class="embed-label">%s</div><div class="embed-row"><code>%s</code><button data-copy="%s">Copy</button></div></div>`, label, v, v)
	}
	links := embedRow("Share Page", fileURL+"/info") + embedRow("Direct Link", fileURL)
	if isImage {
		links += embedRow("HTML", fmt.Sprintf(`<img src="%s" alt="%s">`, fileURL, infoFilename)) +
			embedRow("BBCode", fmt.Sprintf(`[img]%s[/img]`, fileURL)) +
			embedRow("Markdown", fmt.Sprintf(`![%s](%s)`, infoFilename, fileURL))
	}

	// Images open inline; everything else is served as an attachment by /{slug}.
	downloadAttr := ""
	if !isImage {
		downloadAttr = fmt.Sprintf(` download="%s"`, html.EscapeString(infoFilename))
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline' https://cdn.jsdelivr.net; script-src 'unsafe-inline'")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s — pbin</title>
<link rel="stylesheet" href="%s">
%s
<style>
.preview-img{max-width:100%%;border:1px solid var(--pbin-surface-border);border-radius:var(--pbin-radius-lg);margin-bottom:1.5rem}
.file-title{word-break:break-all}
.file-meta{display:flex;flex-wrap:wrap;align-items:center;gap:.5rem 1rem;padding:.6rem 1rem;background:var(--pbin-surface);border:1px solid var(--pbin-surface-border);border-radius:var(--pbin-radius-md);font-size:.85rem;color:var(--pbin-muted);margin-bottom:1.5rem}
.file-meta .meta-sep{width:1px;height:1rem;background:var(--pbin-surface-border)}
.btn-download{display:inline-flex;align-items:center;gap:.5rem;padding:.65rem 1.5rem;border-radius:var(--pbin-radius-md);font-size:.95rem;font-weight:500;text-decoration:none;margin-bottom:2rem}
.btn-download svg{width:18px;height:18px}
.embed-group{margin-bottom:1rem}
.embed-group .embed-label{font-size:.75rem;font-weight:600;text-transform:uppercase;letter-spacing:.05em;color:var(--pbin-muted);margin-bottom:.35rem}
.embed-group .embed-row{display:flex;align-items:center;gap:.5rem}
.embed-group code{flex:1;display:block;background:var(--pbin-surface);border:1px solid var(--pbin-surface-border);padding:.5rem .75rem;border-radius:var(--pbin-radius-sm);font-size:.85rem;word-break:break-all;font-family:ui-monospace,monospace;cursor:pointer}
.embed-group button[data-copy]{font-size:.75rem;padding:.35rem .7rem;border-radius:var(--pbin-radius-sm);border:1px solid var(--pbin-surface-border);background:var(--pbin-surface);cursor:pointer;white-space:nowrap;font-weight:500;transition:background .15s;color:inherit}
.embed-group button[data-copy]:hover{background:var(--pbin-drop-hover-bg)}
</style>
</head>
<body>
%s
<main class="container">
<h2 class="file-title">%s</h2>
%s
<div class="file-meta">%s</div>
<a href="/%s" class="btn-download contrast" role="button"%s><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3"/></svg>Download</a>
%s
</main>
%s
<script>
%s
initCopyButtons();
</script>
</body>
</html>`,
		html.EscapeString(infoFilename),
		picoCSS,
		customCSS,
		viewNavBarHTML(),
		html.EscapeString(infoFilename),
		preview,
		metaHTML,
		slug, downloadAttr,
		links,
		footerHTML,
		sharedJS,
	)
}
