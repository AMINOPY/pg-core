package native

import (
	"context"
	"testing"
)

func TestProfileRejectsTrailingJSONData(t *testing.T) {
	a := New()
	for _, raw := range []string{
		`{"key":"k","server":"edge.example.com:443"} {}`,
		`{"key":"k","server":"edge.example.com:443"} true`,
		`{"key":"k","server":"edge.example.com:443"} trailing`,
	} {
		if err := a.ValidateConfig(context.Background(), []byte(raw)); err == nil {
			t.Errorf("accepted profile with trailing data: %q", raw)
		}
	}
}

func TestProfileAllowsJSONWhitespaceAfterObject(t *testing.T) {
	a := New()
	raw := []byte("{\n  \"key\": \"k\", \"server\": \"edge.example.com:443\"\n}\n\t")
	if err := a.ValidateConfig(context.Background(), raw); err != nil {
		t.Fatalf("valid profile with trailing whitespace rejected: %v", err)
	}
}
