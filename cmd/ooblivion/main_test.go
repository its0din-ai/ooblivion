package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthProbeBypassesCapture(t *testing.T) {
	captured := false
	h := &rootHandler{
		admin:        http.NotFoundHandler(),
		catchAll:     http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { captured = true }),
		healthSecret: "s3cret",
	}

	req := httptest.NewRequest(http.MethodGet, healthPath, nil)
	req.Header.Set(healthHeader, "s3cret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "ok\n" {
		t.Fatalf("body = %q, want %q", got, "ok\n")
	}
	if captured {
		t.Fatal("health probe was captured")
	}
}

func TestIsHealthProbe(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		path   string
		header string
		want   bool
	}{
		{"match", "s3cret", healthPath, "s3cret", true},
		{"no secret configured", "", healthPath, "s3cret", false},
		{"missing header", "s3cret", healthPath, "", false},
		{"wrong header", "s3cret", healthPath, "nope", false},
		{"other path", "s3cret", "/anything", "s3cret", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := &rootHandler{healthSecret: tc.secret}
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.header != "" {
				req.Header.Set(healthHeader, tc.header)
			}
			if got := h.isHealthProbe(req); got != tc.want {
				t.Fatalf("isHealthProbe = %v, want %v", got, tc.want)
			}
		})
	}
}
