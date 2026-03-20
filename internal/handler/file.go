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
	URL       string  `json:"url"`
	DeleteURL string  `json:"delete_url"`
	ExpiresAt *string `json:"expires_at"` // RFC3339 or null
	IsImage   bool    `json:"is_image"`
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
		URL:       result.URL,
		DeleteURL: result.DeleteURL,
		IsImage:   result.IsImage,
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

// Info handles GET /{slug}/info — renders an HTML share page with embed codes for image files.
// For non-image files it redirects to /{slug} for direct download.
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

	// Non-image files: redirect to direct download.
	if !file.IsImage(f.MimeType) {
		http.Redirect(w, r, "/"+slug, http.StatusFound)
		return
	}

	// Build the shareable URL for embed codes.
	// The handler derives the absolute URL from the request (scheme + host).
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	baseURL := scheme + "://" + r.Host
	fileURL := baseURL + "/" + slug
	infoFilename := f.Filename
	if infoFilename == "" {
		infoFilename = slug
	}

	htmlEmbed := fmt.Sprintf(`<img src="%s" alt="%s">`, fileURL, infoFilename)
	bbcodeEmbed := fmt.Sprintf(`[img]%s[/img]`, fileURL)
	markdownEmbed := fmt.Sprintf(`![%s](%s)`, infoFilename, fileURL)

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
<h2>%s</h2>
<img class="preview-img" src="/%s" alt="%s">
<div class="embed-group"><div class="embed-label">HTML</div><div class="embed-row"><code>%s</code><button data-copy="%s">Copy</button></div></div>
<div class="embed-group"><div class="embed-label">BBCode</div><div class="embed-row"><code>%s</code><button data-copy="%s">Copy</button></div></div>
<div class="embed-group"><div class="embed-label">Markdown</div><div class="embed-row"><code>%s</code><button data-copy="%s">Copy</button></div></div>
<div class="embed-group"><div class="embed-label">Direct Link</div><div class="embed-row"><code>%s</code><button data-copy="%s">Copy</button></div></div>
</main>
%s
<script>
function copyText(text){
  if(navigator.clipboard&&navigator.clipboard.writeText){
    navigator.clipboard.writeText(text).catch(function(){fallbackCopy(text);});
  } else {fallbackCopy(text);}
}
function fallbackCopy(text){
  var ta=document.createElement('textarea');
  ta.value=text;ta.style.position='fixed';ta.style.opacity='0';
  document.body.appendChild(ta);ta.select();
  try{document.execCommand('copy');}catch(e){}
  document.body.removeChild(ta);
}
document.querySelectorAll('[data-copy]').forEach(function(btn){
  btn.addEventListener('click', function(){
    copyText(btn.getAttribute('data-copy'));
    var orig = btn.textContent;
    btn.textContent = 'Copied!';
    setTimeout(function(){ btn.textContent = orig; }, 1500);
  });
});
</script>
</body>
</html>`,
		infoFilename,
		picoCSS,
		customCSS,
		viewNavBarHTML(),
		infoFilename,
		slug, infoFilename,
		html.EscapeString(htmlEmbed), html.EscapeString(htmlEmbed),
		html.EscapeString(bbcodeEmbed), html.EscapeString(bbcodeEmbed),
		html.EscapeString(markdownEmbed), html.EscapeString(markdownEmbed),
		html.EscapeString(fileURL), html.EscapeString(fileURL),
		footerHTML,
	)
}
