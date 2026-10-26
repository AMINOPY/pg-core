package native

import (
	"context"
	"encoding/json"
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

func TestProfileRejectsMalformedServerEndpoint(t *testing.T) {
	a := New()
	for _, server := range []string{
		"edge.example.com:abc",
		"edge.example.com:123abc",
		"edge.example.com:0",
		"edge.example.com:65536",
		":443",
		"edge.example.com/path",
		"edge example.com",
	} {
		t.Run(server, func(t *testing.T) {
			raw, err := json.Marshal(Profile{Key: "k", Server: server})
			if err != nil {
				t.Fatal(err)
			}
			if err := a.ValidateConfig(context.Background(), raw); err == nil {
				t.Errorf("accepted malformed server endpoint %q", server)
			}
		})
	}
}

func TestProfileAcceptsValidServerEndpointForms(t *testing.T) {
	a := New()
	for _, server := range []string{
		"edge.example.com",
		"edge.example.com:443",
		"127.0.0.1:8443",
		"[2001:db8::1]:443",
		"2001:db8::1",
	} {
		t.Run(server, func(t *testing.T) {
			raw, err := json.Marshal(Profile{Key: "k", Server: server})
			if err != nil {
				t.Fatal(err)
			}
			if err := a.ValidateConfig(context.Background(), raw); err != nil {
				t.Errorf("rejected valid server endpoint %q: %v", server, err)
			}
		})
	}
}
