package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGzipCompress_CompressResponse(t *testing.T) {
	handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World! This is a test response that should be compressed."))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("expected Content-Encoding: gzip header")
	}

	reader, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	expected := "Hello, World! This is a test response that should be compressed."
	if string(decompressed) != expected {
		t.Errorf("expected %q, got %q", expected, string(decompressed))
	}
}

func TestGzipMiddleware_NoAcceptEncoding(t *testing.T) {
	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("should not set Content-Encoding: gzip")
	}

	if w.Body.String() != "Hello, World!" {
		t.Errorf("expected plain text, got %q", w.Body.String())
	}
}

func TestGzipMiddleware_DecompressRequest(t *testing.T) {
	var receivedBody string
	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))

	originalBody := "This is a compressed request body"
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	gzipWriter.Write([]byte(originalBody))
	gzipWriter.Close()

	req := httptest.NewRequest(http.MethodPost, "/test", &buf)
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if receivedBody != originalBody {
		t.Errorf("expected %q, got %q", originalBody, receivedBody)
	}
}

func TestGzipMiddleware_NoContentEncoding(t *testing.T) {
	var receivedBody string
	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))

	originalBody := "This is a plain request body"
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(originalBody))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if receivedBody != originalBody {
		t.Errorf("expected %q, got %q", originalBody, receivedBody)
	}
}

func TestGzipMiddleware_MultipleWrites(t *testing.T) {
	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("First "))
		w.Write([]byte("Second "))
		w.Write([]byte("Third"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	reader, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	expected := "First Second Third"
	if string(decompressed) != expected {
		t.Errorf("expected %q, got %q", expected, string(decompressed))
	}
}

func TestGzipMiddleware_EmptyResponse(t *testing.T) {
	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestGzipMiddleware_InvalidCompressedRequest(t *testing.T) {
	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Error("expected error reading invalid gzip body")
		}
		w.WriteHeader(http.StatusBadRequest)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("invalid gzip data"))
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGzipMiddleware_LargeResponse(t *testing.T) {
	largeContent := strings.Repeat("A", 10*1024)

	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeContent))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	compressedSize := w.Body.Len()
	if compressedSize >= len(largeContent) {
		t.Errorf("compression not effective: original=%d, compressed=%d", len(largeContent), compressedSize)
	}

	reader, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	if string(decompressed) != largeContent {
		t.Error("decompressed content does not match original")
	}
}

func TestGzipMiddleware_AcceptEncodingWithQuality(t *testing.T) {
	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Test response"))
	}))

	tests := []struct {
		name           string
		acceptEncoding string
		shouldCompress bool
	}{
		{"gzip with quality", "gzip;q=1.0", true},
		{"multiple encodings", "gzip, deflate, br", true},
		{"gzip in middle", "deflate, gzip, br", true},
		{"only deflate", "deflate", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			isCompressed := w.Header().Get("Content-Encoding") == "gzip"
			if isCompressed != tt.shouldCompress {
				t.Errorf("expected compressed=%v, got compressed=%v", tt.shouldCompress, isCompressed)
			}
		})
	}
}

func TestGzipMiddleware_WriteHeader(t *testing.T) {
	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	if w.Header().Get("X-Custom-Header") != "test-value" {
		t.Error("custom header not preserved")
	}

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Content-Encoding header not set")
	}
}

func TestGzipMiddleware_BothRequestAndResponse(t *testing.T) {
	var receivedBody string
	handler := GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		receivedBody = string(body)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response: " + receivedBody))
	}))

	requestBody := "Compressed request"
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	gzipWriter.Write([]byte(requestBody))
	gzipWriter.Close()

	req := httptest.NewRequest(http.MethodPost, "/test", &buf)
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if receivedBody != requestBody {
		t.Errorf("request: expected %q, got %q", requestBody, receivedBody)
	}

	reader, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	expected := "Response: " + requestBody
	if string(decompressed) != expected {
		t.Errorf("response: expected %q, got %q", expected, string(decompressed))
	}
}
