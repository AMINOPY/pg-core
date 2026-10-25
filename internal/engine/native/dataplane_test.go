package native

import "testing"

func TestValidateHealthURL(t *testing.T) {
	for _, raw := range []string{
		"https://example.com/health",
		"http://127.0.0.1:8080/health",
		"",
	} {
		if err := validateHealthURL(raw); err != nil {
			t.Errorf("validateHealthURL(%q): %v", raw, err)
		}
	}
	for _, raw := range []string{
		"example.com/health",
		"http://example.com/health",
		"https://user:secret@example.com/health",
		"file:///etc/passwd",
		"https://example.com:99999/health",
	} {
		if err := validateHealthURL(raw); err == nil {
			t.Errorf("validateHealthURL(%q) unexpectedly succeeded", raw)
		}
	}
}
