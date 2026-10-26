package fusion

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestMeasureRouteQualityCalculatesBoundedMetrics(t *testing.T) {
	route := Route{Path: Path{ID: "primary", Healthy: true, Weight: 1}, Dialer: &net.Dialer{}}
	results := []struct {
		rtt time.Duration
		err error
	}{
		{rtt: 10 * time.Millisecond},
		{err: errors.New("temporary probe failure")},
		{rtt: 30 * time.Millisecond},
		{rtt: 40 * time.Millisecond},
	}
	next := 0
	report, err := MeasureRouteQuality(context.Background(), route, func(context.Context, Route) (time.Duration, error) {
		result := results[next]
		next++
		return result.rtt, result.err
	}, len(results), time.Second)
	if err != nil {
		t.Fatalf("MeasureRouteQuality() error = %v", err)
	}
	if report.Samples != 4 || report.Successes != 3 || report.Failures != 1 {
		t.Fatalf("unexpected sample counts: %+v", report)
	}
	if report.SuccessRate != 0.75 || report.PacketLoss != 0.25 {
		t.Fatalf("unexpected rates: %+v", report)
	}
	if report.MeanRTT != 80*time.Millisecond/3 {
		t.Fatalf("MeanRTT = %s, want %s", report.MeanRTT, 80*time.Millisecond/3)
	}
	if report.MeanJitter != 15*time.Millisecond {
		t.Fatalf("MeanJitter = %s, want 15ms", report.MeanJitter)
	}
}

func TestMeasureRouteQualityRejectsOutOfBoundsInputs(t *testing.T) {
	route := Route{Path: Path{ID: "primary", Weight: 1}, Dialer: &net.Dialer{}}
	probe := func(context.Context, Route) (time.Duration, error) { return time.Millisecond, nil }
	for _, test := range []struct {
		name    string
		samples int
		timeout time.Duration
	}{
		{name: "zero samples", samples: 0, timeout: time.Second},
		{name: "too many samples", samples: maxQualityProbeSamples + 1, timeout: time.Second},
		{name: "zero timeout", samples: 1, timeout: 0},
		{name: "timeout too large", samples: 1, timeout: maxQualityProbeTimeout + time.Nanosecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := MeasureRouteQuality(context.Background(), route, probe, test.samples, test.timeout); err == nil {
				t.Fatal("expected invalid input error")
			}
		})
	}
}

func TestMeasureRouteQualityRejectsRTTAboveTimeout(t *testing.T) {
	route := Route{Path: Path{ID: "primary", Weight: 1}, Dialer: &net.Dialer{}}
	report, err := MeasureRouteQuality(context.Background(), route, func(context.Context, Route) (time.Duration, error) {
		return 2 * time.Second, nil
	}, 1, time.Second)
	if err != nil {
		t.Fatalf("MeasureRouteQuality() error = %v", err)
	}
	if report.Successes != 0 || report.Failures != 1 || report.PacketLoss != 1 {
		t.Fatalf("out-of-bound RTT should count as a failed sample: %+v", report)
	}
}

func TestMeasureRouteQualityHonorsParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	route := Route{Path: Path{ID: "primary", Weight: 1}, Dialer: &net.Dialer{}}
	if _, err := MeasureRouteQuality(ctx, route, func(context.Context, Route) (time.Duration, error) {
		t.Fatal("probe must not run after parent cancellation")
		return 0, nil
	}, 1, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
