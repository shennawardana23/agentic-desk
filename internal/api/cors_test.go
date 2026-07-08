package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCORS_OriginAllowlist pins the 2026-07-07 security fix: only the
// Wails shell / local-dev origins get their origin echoed back; any real
// website's origin gets NO Access-Control-Allow-Origin header, so its
// cross-origin reads of this loopback API fail in the browser. The old
// wildcard "*" would have let any open webpage read profile/memory data.
func TestCORS_OriginAllowlist(t *testing.T) {
	r := NewRouter(Deps{})

	cases := []struct {
		origin  string
		allowed bool
	}{
		{"wails://wails.localhost", true}, // macOS Wails shell (TESTING.md's documented origin)
		{"wails://wails", true},
		{"http://wails.localhost", true},
		{"http://localhost:34115", true}, // wails dev server
		{"http://127.0.0.1:5173", true},  // vite dev server
		{"https://evil.example.com", false},
		{"http://localhost.evil.com", false},
		{"file://", false},
		{"", false},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		got := w.Header().Get("Access-Control-Allow-Origin")
		if c.allowed && got != c.origin {
			t.Errorf("origin %q: ACAO = %q, want echoed back", c.origin, got)
		}
		if !c.allowed && got != "" {
			t.Errorf("origin %q: ACAO = %q, want no header", c.origin, got)
		}
		if got == "*" {
			t.Errorf("origin %q: wildcard ACAO must never come back", c.origin)
		}
	}
}
