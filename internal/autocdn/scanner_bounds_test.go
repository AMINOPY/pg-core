package autocdn

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestStaticSNIWeightBreaksLatencyTies(t *testing.T) {
	catalog := Catalog{
		SNIs:       []string{"a.example", "b.example"},
		CDNIPs:     []string{"1.1.1.1"},
		SNIWeights: map[string]int{"a.example": 10, "b.example": 90},
	}
	routes, err := (Scanner{Workers: 2, Budget: 2}).SelectCandidates(
		context.Background(), catalog,
		func(context.Context, string, string) (time.Duration, error) {
			return 5 * time.Millisecond, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].SNI != "b.example" || routes[0].Weight != 90 {
		t.Fatalf("static weight was not applied as tie-breaker: %+v", routes)
	}
}

func TestCatalogRejectsUnknownOrOutOfRangeWeights(t *testing.T) {
	for _, weights := range []map[string]int{
		{"other.example": 1},
		{"a.example": 1001},
	} {
		c := Catalog{SNIs: []string{"a.example"}, CDNIPs: []string{"1.1.1.1"}, SNIWeights: weights}
		if err := c.Validate(); err == nil {
			t.Errorf("expected invalid weight config: %+v", weights)
		}
	}
}

func TestScannerNeverExceedsMaxWorkers(t *testing.T) {
	snis := make([]string, 40)
	ips := make([]string, 40)
	for i := range snis {
		snis[i] = fmt.Sprintf("sni-%02d.example.com", i)
		ips[i] = fmt.Sprintf("1.1.1.%d", i+1)
	}
	var active, maxActive atomic.Int32
	probe := func(context.Context, string, string) (time.Duration, error) {
		n := active.Add(1)
		for {
			old := maxActive.Load()
			if n <= old || maxActive.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		active.Add(-1)
		return time.Millisecond, nil
	}
	_, err := (Scanner{Workers: 64, Budget: MaxPairProbes}).SelectCandidates(
		context.Background(), Catalog{SNIs: snis, CDNIPs: ips}, probe,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := maxActive.Load(); got > MaxWorkers {
		t.Fatalf("observed %d concurrent probes, limit is %d", got, MaxWorkers)
	}
}
