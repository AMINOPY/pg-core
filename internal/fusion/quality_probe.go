package fusion

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	maxQualityProbeSamples = 20
	maxQualityProbeTimeout = 5 * time.Second
)

// QualityReport summarizes repeated, bounded probes through one route.
// RTT is the arithmetic mean of successful probe RTTs. Jitter is the mean
// absolute difference between consecutive successful RTT samples. Failed
// probes count toward PacketLoss and reduce SuccessRate; they are never
// interpreted as zero-latency successes.
type QualityReport struct {
	Samples     int
	Successes   int
	Failures    int
	SuccessRate float64
	PacketLoss  float64
	MeanRTT     time.Duration
	MeanJitter  time.Duration
}

// MeasureRouteQuality runs a small, sequential probe batch through a route.
// The caller chooses the probe target through the supplied RouteProbe; use a
// controlled/trusted endpoint and a probe that verifies the protocol needed by
// the application. This function does not prove system-wide VPN health and
// does not mutate the router's route snapshots.
//
// Samples must be between 1 and 20. Each probe has its own timeout of at most
// five seconds, bounding work even when a probe stalls. Parent cancellation
// aborts the batch and returns no partial report.
func MeasureRouteQuality(
	ctx context.Context,
	route Route,
	probe RouteProbe,
	samples int,
	timeout time.Duration,
) (QualityReport, error) {
	if ctx == nil {
		return QualityReport{}, errors.New("context is required")
	}
	if route.Path.ID == "" || route.Dialer == nil {
		return QualityReport{}, fmt.Errorf("%w: route ID and dialer are required", ErrInvalidPaths)
	}
	if probe == nil {
		return QualityReport{}, errors.New("route probe is required")
	}
	if samples < 1 || samples > maxQualityProbeSamples {
		return QualityReport{}, fmt.Errorf("sample count must be between 1 and %d", maxQualityProbeSamples)
	}
	if timeout <= 0 || timeout > maxQualityProbeTimeout {
		return QualityReport{}, fmt.Errorf("probe timeout must be positive and at most %s", maxQualityProbeTimeout)
	}

	report := QualityReport{Samples: samples}
	successRTTs := make([]time.Duration, 0, samples)
	for i := 0; i < samples; i++ {
		if err := ctx.Err(); err != nil {
			return QualityReport{}, err
		}
		probeCtx, cancel := context.WithTimeout(ctx, timeout)
		rtt, err := probe(probeCtx, route)
		if err == nil {
			err = probeCtx.Err()
		}
		cancel()
		if ctx.Err() != nil {
			return QualityReport{}, ctx.Err()
		}
		if err != nil || rtt < 0 || rtt > timeout {
			report.Failures++
			continue
		}
		report.Successes++
		successRTTs = append(successRTTs, rtt)
	}

	report.SuccessRate = float64(report.Successes) / float64(report.Samples)
	report.PacketLoss = float64(report.Failures) / float64(report.Samples)
	if len(successRTTs) == 0 {
		return report, nil
	}

	var total time.Duration
	for _, rtt := range successRTTs {
		total += rtt
	}
	report.MeanRTT = total / time.Duration(len(successRTTs))
	if len(successRTTs) > 1 {
		var totalJitter time.Duration
		for i := 1; i < len(successRTTs); i++ {
			delta := successRTTs[i] - successRTTs[i-1]
			if delta < 0 {
				delta = -delta
			}
			totalJitter += delta
		}
		report.MeanJitter = totalJitter / time.Duration(len(successRTTs)-1)
	}
	return report, nil
}
