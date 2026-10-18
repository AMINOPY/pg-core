package diagnostics

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{" Example.COM. ", "example.com", false},
		{"127.0.0.1", "127.0.0.1", false},
		{"", "", true},
		{"https://example.com", "", true},
		{"example.com:443", "", true},
		{"-bad.example", "", true},
		{"bad..example", "", true},
		{strings.Repeat("a", maxRawDomainBytes+1), "", true},
	}
	for _, tt := range tests {
		got, err := NormalizeDomain(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("NormalizeDomain(%q) err=%v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("NormalizeDomain(%q)=%q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCheckManyPreservesOrderAndValidationErrors(t *testing.T) {
	c := Checker{Workers: 100, Timeout: time.Second}
	got := c.CheckMany(context.Background(), []string{"bad domain", "https://invalid", "example.com:443"})
	if len(got) != 3 {
		t.Fatalf("got %d results", len(got))
	}
	for i, result := range got {
		if result.Error == "" {
			t.Errorf("result %d should contain validation error", i)
		}
	}
}

func TestCheckManyEmptyInput(t *testing.T) {
	if got := (Checker{}).CheckMany(context.Background(), nil); len(got) != 0 {
		t.Fatalf("expected empty result, got %d", len(got))
	}
}

func FuzzNormalizeDomainRejectsMalformedInput(f *testing.F) {
	for _, seed := range []string{
		"", "example.com", " Example.COM. ", "https://example.com",
		"example.com:443", "127.0.0.1", "-bad.example", "bad..example",
		"localhost/../../etc/passwd", "user@host", "[::1]", "\x00",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > maxRawDomainBytes {
			return
		}
		host, err := NormalizeDomain(raw)
		if err != nil {
			return
		}
		if host == "" || len(host) > 253 || strings.ContainsAny(host, "/:@?# ") {
			t.Fatalf("invalid normalized hostname returned: %q", host)
		}
	})
}
