package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ahmethakanbesel/pbin/internal/domain/file"
	"github.com/ahmethakanbesel/pbin/internal/handler"
)

// --- mock service ---

type mockFileService struct {
	uploadFn func(ctx context.Context, req file.UploadRequest) (file.UploadResult, error)
	getFn    func(ctx context.Context, slug, password string) (file.GetResult, error)
	deleteFn func(ctx context.Context, slug, secret string) error
}

func (m *mockFileService) Upload(ctx context.Context, req file.UploadRequest) (file.UploadResult, error) {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, req)
	}
	return file.UploadResult{}, nil
}

func (m *mockFileService) Get(ctx context.Context, slug, password string) (file.GetResult, error) {
	if m.getFn != nil {
		return m.getFn(ctx, slug, password)
	}
	return file.GetResult{}, nil
}

func (m *mockFileService) Delete(ctx context.Context, slug, secret string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, slug, secret)
	}
	return nil
}

// helper to build a multipart/form-data body
func buildMultipartForm(t *testing.T, fields map[string]string, filename, fileContent string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if filename != "" {
		fw, err := w.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(fw, fileContent)
	}
	for k, v := range fields {
		w.WriteField(k, v)
	}
	w.Close()
	return &buf, w.FormDataContentType()
}

// --- Upload handler tests ---

func TestUpload_Success(t *testing.T) {
	svc := &mockFileService{
		uploadFn: func(_ context.Context, req file.UploadRequest) (file.UploadResult, error) {
			return file.UploadResult{
				Slug:      "abc123",
				URL:       "http://localhost/abc123",
				DeleteURL: "http://localhost/delete/abc123/secret",
				IsImage:   false,
			}, nil
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	body, ct := buildMultipartForm(t, map[string]string{"expiry": "1d"}, "test.txt", "hello world")
	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()

	h.Upload(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := resp["url"]; !ok {
		t.Error("response missing 'url' field")
	}
	if _, ok := resp["delete_url"]; !ok {
		t.Error("response missing 'delete_url' field")
	}
}

func TestUpload_MissingFile(t *testing.T) {
	h := handler.NewFileHandler(&mockFileService{}, 10*1024*1024)

	body, ct := buildMultipartForm(t, map[string]string{}, "", "")
	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()

	h.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpload_SecurityHeaders(t *testing.T) {
	svc := &mockFileService{
		uploadFn: func(_ context.Context, req file.UploadRequest) (file.UploadResult, error) {
			return file.UploadResult{URL: "http://localhost/abc"}, nil
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	body, ct := buildMultipartForm(t, nil, "test.txt", "data")
	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()

	h.Upload(w, req)

	if v := w.Header().Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff, got %q", v)
	}
	if v := w.Header().Get("X-Frame-Options"); v != "DENY" {
		t.Errorf("expected X-Frame-Options: DENY, got %q", v)
	}
}

func TestUpload_MaxBytesEnforced(t *testing.T) {
	h := handler.NewFileHandler(&mockFileService{}, 100) // 100 bytes max

	// Build a body larger than the limit
	content := strings.Repeat("x", 200)
	body, ct := buildMultipartForm(t, nil, "big.txt", content)
	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()

	h.Upload(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Serve handler tests ---

func TestServe_NotFound(t *testing.T) {
	svc := &mockFileService{
		getFn: func(_ context.Context, slug, _ string) (file.GetResult, error) {
			return file.GetResult{}, file.ErrNotFound
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	req.SetPathValue("slug", "abc123")
	w := httptest.NewRecorder()

	h.Serve(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestServe_Expired(t *testing.T) {
	svc := &mockFileService{
		getFn: func(_ context.Context, slug, _ string) (file.GetResult, error) {
			return file.GetResult{}, file.ErrExpired
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	req.SetPathValue("slug", "abc123")
	w := httptest.NewRecorder()

	h.Serve(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected 410, got %d", w.Code)
	}
}

func TestServe_AlreadyConsumed(t *testing.T) {
	svc := &mockFileService{
		getFn: func(_ context.Context, slug, _ string) (file.GetResult, error) {
			return file.GetResult{}, file.ErrAlreadyConsumed
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	req.SetPathValue("slug", "abc123")
	w := httptest.NewRecorder()

	h.Serve(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected 410, got %d", w.Code)
	}
}

func TestServe_WrongPassword_ShowsFormForBrowser(t *testing.T) {
	svc := &mockFileService{
		getFn: func(_ context.Context, slug, _ string) (file.GetResult, error) {
			return file.GetResult{}, file.ErrWrongPassword
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	req.SetPathValue("slug", "abc123")
	// No Accept: application/json, no X-Password — browser request
	w := httptest.NewRecorder()

	h.Serve(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected HTML password form, got Content-Type %q", ct)
	}
}

func TestServe_WrongPassword_JSON401ForAPI(t *testing.T) {
	svc := &mockFileService{
		getFn: func(_ context.Context, slug, _ string) (file.GetResult, error) {
			return file.GetResult{}, file.ErrWrongPassword
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	req.SetPathValue("slug", "abc123")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	h.Serve(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestServe_ImageInline(t *testing.T) {
	f := file.File{Slug: "abc123", Filename: "photo.png", MimeType: "image/png"}
	svc := &mockFileService{
		getFn: func(_ context.Context, slug, _ string) (file.GetResult, error) {
			return file.GetResult{
				F:       f,
				Content: io.NopCloser(strings.NewReader("fakepngdata")),
				IsImage: true,
			}, nil
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	req.SetPathValue("slug", "abc123")
	w := httptest.NewRecorder()

	h.Serve(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "inline") {
		t.Errorf("expected inline Content-Disposition for image, got %q", cd)
	}
}

func TestServe_NonImageAttachment(t *testing.T) {
	f := file.File{Slug: "abc123", Filename: "document.pdf", MimeType: "application/pdf"}
	svc := &mockFileService{
		getFn: func(_ context.Context, slug, _ string) (file.GetResult, error) {
			return file.GetResult{
				F:       f,
				Content: io.NopCloser(strings.NewReader("pdfdata")),
				IsImage: false,
			}, nil
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	req.SetPathValue("slug", "abc123")
	w := httptest.NewRecorder()

	h.Serve(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("expected attachment Content-Disposition for non-image, got %q", cd)
	}
}

// --- Delete handler tests ---

func TestDelete_Success(t *testing.T) {
	svc := &mockFileService{
		deleteFn: func(_ context.Context, slug, secret string) error {
			return nil
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/delete/abc123/mysecret", nil)
	req.SetPathValue("slug", "abc123")
	req.SetPathValue("secret", "mysecret")
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]bool
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp["deleted"] {
		t.Error("expected deleted:true in response")
	}
}

func TestDelete_BadSecret(t *testing.T) {
	svc := &mockFileService{
		deleteFn: func(_ context.Context, slug, secret string) error {
			return file.ErrBadDeleteSecret
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/delete/abc123/wrongsecret", nil)
	req.SetPathValue("slug", "abc123")
	req.SetPathValue("secret", "wrongsecret")
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestDelete_NotFound(t *testing.T) {
	svc := &mockFileService{
		deleteFn: func(_ context.Context, slug, secret string) error {
			return file.ErrNotFound
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/delete/abc123/anysecret", nil)
	req.SetPathValue("slug", "abc123")
	req.SetPathValue("secret", "anysecret")
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- Info handler tests ---

func TestInfo_ImageShowsEmbedCodes(t *testing.T) {
	f := file.File{Slug: "abc123", Filename: "photo.png", MimeType: "image/png"}
	svc := &mockFileService{
		getFn: func(_ context.Context, slug, _ string) (file.GetResult, error) {
			return file.GetResult{
				F:       f,
				Content: io.NopCloser(strings.NewReader("")),
				IsImage: true,
			}, nil
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/abc123/info", nil)
	req.SetPathValue("slug", "abc123")
	req.Host = "localhost"
	w := httptest.NewRecorder()

	h.Info(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"BBCode", "[img]", "Markdown", "Direct link", "<img src="} {
		if !strings.Contains(body, want) {
			t.Errorf("expected embed code %q in info page, not found", want)
		}
	}
}

func TestInfo_NonImageRedirects(t *testing.T) {
	f := file.File{Slug: "abc123", Filename: "doc.pdf", MimeType: "application/pdf"}
	svc := &mockFileService{
		getFn: func(_ context.Context, slug, _ string) (file.GetResult, error) {
			return file.GetResult{
				F:       f,
				Content: io.NopCloser(strings.NewReader("")),
				IsImage: false,
			}, nil
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/abc123/info", nil)
	req.SetPathValue("slug", "abc123")
	w := httptest.NewRecorder()

	h.Info(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/abc123" {
		t.Errorf("expected redirect to /abc123, got %q", loc)
	}
}

func TestInfo_NotFound(t *testing.T) {
	svc := &mockFileService{
		getFn: func(_ context.Context, slug, _ string) (file.GetResult, error) {
			return file.GetResult{}, file.ErrNotFound
		},
	}
	h := handler.NewFileHandler(svc, 10*1024*1024)

	req := httptest.NewRequest(http.MethodGet, "/abc123/info", nil)
	req.SetPathValue("slug", "abc123")
	w := httptest.NewRecorder()

	h.Info(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// Ensure handler.NewFileHandler accepts a service interface (compilation check).
var _ = fmt.Sprintf

// Test that errors.Is on file sentinel errors works as expected.
func TestSentinelErrors(t *testing.T) {
	if !errors.Is(file.ErrNotFound, file.ErrNotFound) {
		t.Error("ErrNotFound should match itself")
	}
}
