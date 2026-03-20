package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/domain/paste"
)

// PasteService is the interface the paste handler depends on.
type PasteService interface {
	Create(ctx context.Context, req paste.CreateRequest) (paste.CreateResult, error)
	Get(ctx context.Context, shareSlug, passwordAttempt string) (paste.GetResult, error)
	Delete(ctx context.Context, shareSlug, deleteSecret string) error
}

// PasteHandler handles paste create, view, raw, and delete operations.
type PasteHandler struct {
	svc PasteService
}

// NewPasteHandler constructs a PasteHandler.
func NewPasteHandler(svc PasteService) *PasteHandler {
	return &PasteHandler{svc: svc}
}

// pasteCreateRequest is the JSON body accepted by Create.
type pasteCreateRequest struct {
	Content  string `json:"content"`
	Title    string `json:"title"`
	Lang     string `json:"lang"`
	Expiry   string `json:"expiry"`
	Password string `json:"password"`
	OneUse   bool   `json:"one_use"`
}

// pasteCreateResponse is the JSON shape returned after a successful paste creation.
type pasteCreateResponse struct {
	URL       string  `json:"url"`
	DeleteURL string  `json:"delete_url"`
	ExpiresAt *string `json:"expires_at"` // RFC3339 or null
}

// Create handles POST /api/paste — accepts JSON body and creates a new paste.
func (h *PasteHandler) Create(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)

	var req pasteCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content must not be empty")
		return
	}

	if req.Expiry == "" {
		req.Expiry = "1d"
	}

	result, err := h.svc.Create(r.Context(), paste.CreateRequest{
		Title:    req.Title,
		Content:  req.Content,
		Lang:     req.Lang,
		Expiry:   req.Expiry,
		Password: req.Password,
		OneUse:   req.OneUse,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "paste creation failed")
		return
	}

	resp := pasteCreateResponse{
		URL:       result.URL,
		DeleteURL: result.DeleteURL,
	}
	if result.ExpiresAt != nil {
		s := result.ExpiresAt.UTC().Format(time.RFC3339)
		resp.ExpiresAt = &s
	}

	writeJSON(w, http.StatusCreated, resp)
}

// generateNonce generates a cryptographically random 16-byte nonce as a base64 string.
func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// View handles GET /{slug} (paste) — renders HTML with syntax highlighting via highlight.js.
func (h *PasteHandler) View(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.svc.Get(r.Context(), slug, password)
	if err != nil {
		switch {
		case errors.Is(err, paste.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		case errors.Is(err, paste.ErrExpired):
			writeError(w, http.StatusGone, "paste has expired")
		case errors.Is(err, paste.ErrAlreadyConsumed):
			writeError(w, http.StatusGone, "paste has already been viewed")
		case errors.Is(err, paste.ErrWrongPassword):
			// Show password form for browsers; 401 JSON for API clients.
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

	nonce, err := generateNonce()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// CSP: allow highlight.js CDN for script and style, Pico CSS from jsdelivr, inline script/style via nonce.
	csp := fmt.Sprintf(
		"default-src 'none'; script-src 'nonce-%s' https://cdnjs.cloudflare.com; style-src 'nonce-%s' https://cdn.jsdelivr.net https://cdnjs.cloudflare.com; img-src 'none'",
		nonce, nonce,
	)
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	p := result.P

	// Build metadata strings.
	title := p.Title
	lang := p.Lang

	expiryInfo := "Never expires"
	if result.ExpiresAt != nil {
		expiryInfo = "Expires: " + result.ExpiresAt.UTC().Format(time.RFC3339)
	}

	oneUseInfo := ""
	if p.OneUse {
		oneUseInfo = `<span class="meta-item meta-oneuse">One-use</span>`
	}

	titleHTML := ""
	if title != "" {
		titleHTML = fmt.Sprintf(`<h2 style="margin-bottom:.5rem">%s</h2>`, html.EscapeString(title))
	}

	// HTML-escape paste content to prevent XSS.
	escapedContent := html.EscapeString(p.Content)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s — pbin</title>
<link rel="stylesheet" href="%s">
<link rel="stylesheet" media="(prefers-color-scheme: light)"
      href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.11.1/styles/github.min.css">
<link rel="stylesheet" media="(prefers-color-scheme: dark)"
      href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.11.1/styles/atom-one-dark.min.css">
%s
<style nonce="%s">
.meta-bar{display:flex;flex-wrap:wrap;gap:.75rem;align-items:center;font-size:.85rem;color:var(--pbin-muted);margin-bottom:.75rem;padding:.6rem 1rem;background:var(--pbin-surface);border-radius:var(--pbin-radius-md);border:1px solid var(--pbin-surface-border)}
.meta-item{white-space:nowrap}
.meta-lang{font-weight:600;text-transform:uppercase;font-size:.75rem;letter-spacing:.05em}
.meta-oneuse{color:#d73a49;font-weight:600}
@media(prefers-color-scheme:dark){.meta-oneuse{color:#e06c75}}
.actions{display:flex;gap:.5rem;margin-left:auto}
.actions a,.actions button{padding:.3rem .7rem;font-size:.8rem;text-decoration:none;border:1px solid var(--pbin-surface-border);border-radius:var(--pbin-radius-sm);background:transparent;color:inherit;cursor:pointer;font-family:inherit;font-weight:500;transition:background .15s}
.actions a:hover,.actions button:hover{background:var(--pbin-drop-hover-bg)}
.code-wrap{position:relative}
pre{margin:0;padding:0;border-radius:var(--pbin-radius-lg);overflow:auto;border:1px solid var(--pbin-surface-border);counter-reset:line}
pre code.hljs{padding:1rem 1rem 1rem 3.5rem;display:block;line-height:1.6;white-space:pre}
pre code.hljs .line{display:block;position:relative}
pre code.hljs .line::before{counter-increment:line;content:counter(line);position:absolute;left:-3rem;width:2.5rem;text-align:right;color:var(--pbin-muted);user-select:none;font-size:.85em}
</style>
</head>
<body>
%s
<main>
%s
<div class="meta-bar">
  <span class="meta-item meta-lang">%s</span>
  <span class="meta-item">%s</span>
  %s
  <div class="actions">
    <button onclick="navigator.clipboard.writeText(document.querySelector('code').textContent);var b=this;b.textContent='Copied!';setTimeout(function(){b.textContent='Copy';},1500)">Copy</button>
    <a href="/%s/raw">Raw</a>
  </div>
</div>
<div class="code-wrap">
<pre><code class="language-%s">%s</code></pre>
</div>
</main>
%s
<script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.11.1/highlight.min.js"></script>
<script nonce="%s">
(function(){
  var code = document.querySelector('pre code');
  if(code){
    var lines = code.textContent.split('\n');
    // Remove trailing empty line from final newline
    if(lines.length > 0 && lines[lines.length-1] === ''){lines.pop();}
    code.innerHTML = lines.map(function(l){
      return '<span class="line">'+hljs.highlight(l,{language:code.className.replace('language-','').replace('hljs ','').split(' ')[0]||'plaintext',ignoreIllegals:true}).value+'</span>';
    }).join('\n');
  }
  hljs.highlightAll();
})();
</script>
</body>
</html>`,
		html.EscapeString(func() string {
			if title != "" {
				return title
			}
			return slug
		}()),
		picoCSS,
		customCSS,
		nonce,
		viewNavBarHTML(),
		titleHTML,
		html.EscapeString(lang),
		html.EscapeString(expiryInfo),
		oneUseInfo,
		html.EscapeString(slug),
		html.EscapeString(lang),
		escapedContent,
		footerHTML,
		nonce,
	)
}

// Raw handles GET /{slug}/raw — returns paste content as plain text.
func (h *PasteHandler) Raw(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.svc.Get(r.Context(), slug, password)
	if err != nil {
		switch {
		case errors.Is(err, paste.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		case errors.Is(err, paste.ErrExpired):
			writeError(w, http.StatusGone, "paste has expired")
		case errors.Is(err, paste.ErrAlreadyConsumed):
			writeError(w, http.StatusGone, "paste has already been viewed")
		case errors.Is(err, paste.ErrWrongPassword):
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

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s.txt"`, slug))
	fmt.Fprint(w, result.P.Content)
}

// Delete handles GET /delete/{slug}/{secret} — removes a paste.
func (h *PasteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)

	slug := r.PathValue("slug")
	secret := r.PathValue("secret")

	if slug == "" || secret == "" {
		writeError(w, http.StatusBadRequest, "missing slug or secret")
		return
	}

	if err := h.svc.Delete(r.Context(), slug, secret); err != nil {
		switch {
		case errors.Is(err, paste.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		case errors.Is(err, paste.ErrBadDeleteSecret):
			writeError(w, http.StatusForbidden, "invalid delete token")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}
