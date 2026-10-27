package xray

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/autocdn"
)

func TestPrepareAutoCDNProfileSelectsMeasuredPairAndPreservesProfile(t *testing.T) {
	profile := []byte(\`{"outbounds":[{"tag":"proxy","protocol":"vless","settings":{"vnext":[{"address":"origin.example.com","port":443,"users":[{"id":"keep-me"}]}]},"streamSettings":{"network":"ws","security":"tls","tlsSettings":{"serverName":"origin.example.com"},"wsSettings":{"path":"/tunnel","headers":{"Host":"origin.example.com"}}}}]}\`)
	catalog := autocdn.Catalog{
		SNIs:   []string{"slow.example.com", "fast.example.com"},
		CDNIPs: []string{"1.1.1.1"},
	}
	prepared, route, err := PrepareAutoCDNProfile(context.Background(), profile, catalog, autocdn.Scanner{Workers: 2, Budget: 2},
		func(_ context.Context, sni, _ string) (time.Duration, error) {
			if strings.HasPrefix(sni, "fast.") {
				return 5 * time.Millisecond, nil
			}
			return 50 * time.Millisecond, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if route.SNI != "fast.example.com" || route.CDNIP != "1.1.1.1" {
		t.Fatalf("unexpected selected route: %+v", route)
	}
	var root map[string]any
	if err := json.Unmarshal(prepared, &root); err != nil {
		t.Fatal(err)
	}
	out := root["outbounds"].([]any)[0].(map[string]any)
	server := out["settings"].(map[string]any)["vnext"].([]any)[0].(map[string]any)
	stream := out["streamSettings"].(map[string]any)
	if server["address"] != "1.1.1.1" {
		t.Fatalf("edge IP not applied: %v", server["address"])
	}
	if stream["tlsSettings"].(map[string]any)["serverName"] != "fast.example.com" {
		t.Fatalf("SNI not applied: %v", stream["tlsSettings"])
	}
	if server["users"].([]any)[0].(map[string]any)["id"] != "keep-me" {
		t.Fatal("profile credentials changed")
	}
	if stream["wsSettings"].(map[string]any)["path"] != "/tunnel" {
		t.Fatal("transport path changed")
	}
}

func TestPrepareAutoCDNProfileRequiresExistingXrayProfile(t *testing.T) {
	_, _, err := PrepareAutoCDNProfile(context.Background(), nil,
		autocdn.Catalog{SNIs: []string{"cdn.example.com"}, CDNIPs: []string{"1.1.1.1"}},
		autocdn.Scanner{}, func(context.Context, string, string) (time.Duration, error) {
			return time.Millisecond, nil
		})
	if err == nil || !strings.Contains(err.Error(), "existing Xray profile is required") {
		t.Fatalf("expected explicit missing-profile error, got %v", err)
	}
}

func TestPrepareAutoCDNProfileRejectsUnusableCatalog(t *testing.T) {
	_, _, err := PrepareAutoCDNProfile(context.Background(), []byte(\`{"outbounds":[]}\`),
		autocdn.Catalog{}, autocdn.Scanner{}, nil)
	if err == nil {
		t.Fatal("expected invalid catalog error")
	}
}
