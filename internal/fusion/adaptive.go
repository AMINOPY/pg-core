package fusion

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// AdaptiveMode is the action selected from measured path health. DualFlow
// distributes new connections across healthy paths; it does not split one
// connection's byte stream across paths.
type AdaptiveMode string

const (
	AdaptiveOffline	AdaptiveMode = "offline"
	AdaptivePrimary	AdaptiveMode = "primary"
	AdaptiveDualFlow	AdaptiveMode = "dual-flow"
	AdaptiveBackup   AdaptiveMode = "backup"
)

// AdaptiveThresholds are explicit policy limits. They describe observed
// reachability/quality, not a definitive DPI detector.
type AdaptiveThresholds struct {
	MinSuccessRate float64
	MaxPacketLoss  float64
	MaxMeanRTT     time.Duration
}

// AdaptivePath is a route and its recent bounded quality measurement.
type AdaptivePath struct {
	Route	Route
	Quality	QualityReport
}

// AdaptiveDecision explains which path(s) should carry newly opened flows.
type AdaptiveDecision struct {
	Mode	AdaptiveMode
	PrimaryID string
	BackupID  string
	Reason    string
}

// DecideAdaptivePaths chooses a primary, a warm backup, or flow-level
// distribution from caller-supplied path measurements. primaryID is the
// preferred route when healthy. primaryBlocked should only be set when the
// caller has explicit evidence that the primary cannot reach its target.
// This policy never claims to identify DPI from latency alone.
func DecideAdaptivePaths(
	primaryID string,
	paths []AdaptivePath,
	thresholds AdaptiveThresholds,
	enableDualFlow bool,
	primaryBlocked bool,
) (AdaptiveDecision, error) {
	if len(paths) == 0 {
		return AdaptiveDecision{Mode: AdaptiveOffline, Reason: "no paths supplied"}, nil
	}
	if primaryID == "" {
		return AdaptiveDecision{}, errors.New("primary path ID is required")
	}
	if math.IsNaN(thresholds.MinSuccessRate) || math.IsInf(thresholds.MinSuccessRate, 0) ||
		thresholds.MinSuccessRate < 0 || thresholds.MinSuccessRate > 1 ||
		math.IsNaN(thresholds.MaxPacketLoss) || math.IsInf(thresholds.MaxPacketLoss, 0) ||
		thresholds.MaxPacketLoss < 0 || thresholds.MaxPacketLoss > 1 ||
		thresholds.MaxMeanRTT < 0 {
		return AdaptiveDecision{}, errors.New("invalid adaptive thresholds")
	}

	byID := make(map[string]AdaptivePath, len(paths))
	for _, p := range paths {
		id := p.Route.Path.ID
		if id == "" || p.Route.Dialer == nil {
			return AdaptiveDecision{}, fmt.Errorf("%w: each adaptive path needs an ID and dialer", ErrInvalidPaths)
		}
		if _, exists := byID[id]; exists {
			return AdaptiveDecision{}, fmt.Errorf("%w: duplicate path ID %q", ErrInvalidPaths, id)
		}
		q := p.Quality
		if q.Samples < 0 || q.Successes < 0 || q.Failures < 0 ||
			q.Successes+q.Failures > q.Samples || q.MeanRTT < 0 || q.MeanJitter < 0 ||
			math.IsNaN(q.SuccessRate) || math.IsInf(q.SuccessRate, 0) ||
			q.SuccessRate < 0 || q.SuccessRate > 1 ||
			math.IsNaN(q.PacketLoss) || math.IsInf(q.PacketLoss, 0) ||
			q.PacketLoss < 0 || q.PacketLoss > 1 {
			return AdaptiveDecision{}, fmt.Errorf("%w: invalid quality report for %q", ErrInvalidPaths, id)
		}
		byID[id] = p
	}
	primary, exists := byID[primaryID]
	if !exists {
		return AdaptiveDecision{}, fmt.Errorf("%w: primary path %q is missing", ErrInvalidPaths, primaryID)
	}
	usable := func(p AdaptivePath) bool {
		q := p.Quality
		if !p.Route.Path.Healthy || q.Samples == 0 || q.SuccessRate < thresholds.MinSuccessRate ||
			q.PacketLoss > thresholds.MaxPacketLoss {
			return false
		}
		return thresholds.MaxMeanRTT == 0 || q.MeanRTT <= thresholds.MaxMeanRTT
	}

	backupID := ""
	for _, p := range paths {
		if p.Route.Path.ID == primaryID || !usable(p) {
			continue
		}
		if backupID == "" || p.Quality.MeanRTT < byID[backupID].Quality.MeanRTT ||
			(p.Quality.MeanRTT == byID[backupID].Quality.MeanRTT && p.Route.Path.ID < backupID) {
			backupID = p.Route.Path.ID
		}
	}
	if primaryBlocked || !usable(primary) {
		if backupID == "" {
			return AdaptiveDecision{Mode: AdaptiveOffline, PrimaryID: primaryID, Reason: "primary unavailable and no healthy backup"}, nil
		}
		return AdaptiveDecision{Mode: AdaptiveBackup, PrimaryID: primaryID, BackupID: backupID, Reason: "primary blocked or below configured quality thresholds; use healthy backup"}, nil
	}
	if enableDualFlow && backupID != "" {
		return AdaptiveDecision{Mode: AdaptiveDualFlow, PrimaryID: primaryID, BackupID: backupID, Reason: "both paths are healthy; distribute new flows using the fusion router"}, nil
	}
	return AdaptiveDecision{Mode: AdaptivePrimary, PrimaryID: primaryID, BackupID: backupID, Reason: "primary is healthy; keep backup ready"}, nil
}
