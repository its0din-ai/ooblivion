package admin

import (
	"strings"
	"testing"
)

func TestIsDisplayableText(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"empty", "", true},
		{"plain ascii", "GET / HTTP/1.1", true},
		{"json with whitespace", "{\n\t\"a\": 1\r\n}", true},
		{"nul byte", "abc\x00def", false},
		{"binary high bytes", "abc\xffdef", false},
		{"utf8 text", "halo dunia — ünïcode", false},
		{"escape byte", "abc\x1b[31m", false},
		{"del byte", "abc\x7f", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDisplayableText(tc.body); got != tc.want {
				t.Fatalf("isDisplayableText(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

func TestBodyDisplayState(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantHidden bool
		wantReason string
	}{
		{"empty body", "", false, ""},
		{"small text", "hello", false, ""},
		{"exactly at limit", strings.Repeat("a", maxDisplayBody), false, ""},
		{"over limit", strings.Repeat("a", maxDisplayBody+1), true, "body is too large to display inline"},
		{"binary", "PK\x03\x04", true, "body contains binary or non-ASCII data"},
		{"binary and huge", strings.Repeat("a", maxDisplayBody) + "\xff", true, "body contains binary or non-ASCII data and is too large to display inline"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hidden, reason := bodyDisplayState(tc.body)
			if hidden != tc.wantHidden || reason != tc.wantReason {
				t.Fatalf("bodyDisplayState(len=%d) = (%v, %q), want (%v, %q)",
					len(tc.body), hidden, reason, tc.wantHidden, tc.wantReason)
			}
		})
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{154208, "150.6 KiB"},
		{1 << 20, "1.0 MiB"},
	}
	for _, tc := range tests {
		if got := humanBytes(tc.in); got != tc.want {
			t.Fatalf("humanBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBodyFileExt(t *testing.T) {
	tests := map[string]string{
		"application/json":                  ".json",
		"application/json; charset=utf-8":   ".json",
		"application/xml":                   ".xml",
		"text/csv":                          ".csv",
		"application/javascript":            ".js",
		"text/html; charset=utf-8":          ".html",
		"application/x-www-form-urlencoded": ".txt",
		"text/plain":                        ".txt",
		"application/pdf":                   ".pdf",
		"image/png":                         ".png",
		"image/jpeg":                        ".jpg",
		"application/zip":                   ".zip",
		"application/octet-stream":          ".bin",
		"":                                  ".bin",
	}
	for ct, want := range tests {
		if got := bodyFileExt(ct); got != want {
			t.Fatalf("bodyFileExt(%q) = %q, want %q", ct, got, want)
		}
	}
}
