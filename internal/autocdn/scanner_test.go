package autocdn

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestCatalogValidate(t *testing.T) {
	good := Catalog{SNIs: []string{"cdn.example.com"}, CDNIPs: []string{"1.1.1.1"}}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := []Catalog{
		{},
		{SNIs: []string{"not a hostname"}, CDNIPs: []string{"1.1.1.1"}},
		{SNIs: []string{"1.1.1.1"}, CDNIPs: []string{"1.1.1.1"}},
		{SNIs: []string{"cdn.example.com"}, CDNIPs: []string{"127.0.0.1"}},
		{SNIs: []string{"cdn.example.com"}, CDNIPs: []string{"not-an-ip"}},
	}
	for _, c := range bad {
		if err := c.Validate(); err == nil {
			t.Errorf("expected invalid catalog: %+v", c)
		}
	}
}

func TestMakePairsIsBoundedAndCoversEachCandidateOnFirstSweep(t *testing.T) {
	c := Catalog{SNIs: []string{"a.example", "b.example", "c.example"}, CDNIPs: []string{"1.1.1.1", "1.0.0.1"}}
	pairs := makePairs(c, 4)
	if len(pairs) != 4 {
		t.Fatalf("got %d pairs, want 4", len(pairs))
	}
	snis, ips := map[string]bool{}, map[string]bool{}
	for _, p := range pairs {
		snis[p.SNI], ips[p.CDNIP] = true, true
	}
	if len(snis) != 3 || len(ips) != 2 {
		t.Fatalf("first sweep did not cover candidates: snis=%v ips=%v", snis, ips)
	}
}

func TestSelectPicksLowestLatencySuccessfulRoute(t *testing.T) {
	c := Catalog{SNIs: []string{"a.example", "b.example"}, CDNIPs: []string{"1.1.1.1", "1.0.0.1"}}
	scanner := Scanner{Workers: 2, Budget: 4}
	got, err := scanner.Select(context.Background(), c, func(_ context.Context, sni, ip string) (time.Duration, error) {
		if sni == "a.example" && ip == "1.1.1.1" {
			return 10 * time.Millisecond, nil
		}
		if sni == "b.example" && ip == "1.0.0.1" {
			return 3 * time.Millisecond, nil
		}
		return 0, errors.New("unhealthy")
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.SNI != "b.example" || got.CDNIP != "1.0.0.1" || got.Latency != 3*time.Millisecond {
		t.Fatalf("unexpected route: %+v", got)
	}
}

func TestSelectReturnsErrorWhenNoPairWorks(t *testing.T) {
	c := Catalog{SNIs: []string{"a.example"}, CDNIPs: []string{"1.1.1.1"}}
	_, err := (Scanner{}).Select(context.Background(), c, func(context.Context, string, string) (time.Duration, error) {
		return 0, errors.New("no route")
	})
	if err == nil {
		t.Fatal("expected no-route error")
	}
}

func TestSelectCandidatesReturnsAtMostTwentyRoutes(t *testing.T) {
	snis := make([]string, 30)
	ips := make([]string, 30)
	for i := range snis {
		snis[i] = fmt.Sprintf("sni-%02d.example.com", i)
		ips[i] = fmt.Sprintf("1.1.1.%d", i+1)
	}
	c := Catalog{SNIs: snis, CDNIPs: ips}
	routes, err := (Scanner{Workers: 8, Budget: MaxPairProbes}).SelectCandidates(context.Background(), c,
		func(_ context.Context, sni, ip string) (time.Duration, error) {
			return time.Duration(len(sni)+len(ip)) * time.Millisecond, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != MaxRankedRoutes {
		t.Fatalf("got %d routes, want %d", len(routes), MaxRankedRoutes)
	}
	for i := 1; i < len(routes); i++ {
		if routes[i].Latency < routes[i-1].Latency {
			t.Fatalf("routes are not ranked: %v then %v", routes[i-1], routes[i])
		}
	}
}

func TestSelectRejectsNegativeProbeLatency(t *testing.T) {
	catalog := Catalog{SNIs: []string{"a.example"}, CDNIPs: []string{"1.1.1.1"}}
	_, err := (Scanner{}).Select(context.Background(), catalog, func(context.Context, string, string) (time.Duration, error) {
		return -time.Millisecond, nil
	})
	if err == nil {
		t.Fatal("negative latency must not be treated as a healthy route")
	}
}
