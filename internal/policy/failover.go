package policy

import (
	"errors"
	"sort"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
)

var ErrInvalidFailoverConfig = errors.New("invalid failover policy configuration")

// ServerCandidate identifies a server profile and its subscription/source.
// A negative latency (including a sentinel such as -1) means no valid sample.
type ServerCandidate struct {
	ID             string
	SubscriptionID string
	Status         engine.Status
	Latency        time.Duration
}

type FailoverConfig struct {
	FailureThreshold int
	MaxLatency       time.Duration
	Cooldown         time.Duration
	MinImprovement   time.Duration
}

type FailoverDecision struct {
	CurrentID string
	Target    ServerCandidate
	Switch    bool
	Reason    string
}

// FailoverController provides deterministic failover decisions. It does not
// start/stop engines or modify routes; callers must execute and verify a switch.
type FailoverController struct {
	cfg          FailoverConfig
	currentID    string
	failureStreak int
	lastSwitch   time.Time
}

func NewFailoverController(cfg FailoverConfig) (*FailoverController, error) {
	if cfg.FailureThreshold < 1 || cfg.MaxLatency <= 0 || cfg.Cooldown < 0 || cfg.MinImprovement < 0 {
		return nil, ErrInvalidFailoverConfig
	}
	return &FailoverController{cfg: cfg}, nil
}

// Evaluate observes one health sample and returns a switch decision. Calls
// should be serialized by the owner of the active connection. now must be
// monotonic-compatible with the timestamps supplied by the caller.
func (c *FailoverController) Evaluate(currentID string, candidates []ServerCandidate, now time.Time) (FailoverDecision, error) {
	if currentID == "" {
		return FailoverDecision{}, errors.New("current server ID is required")
	}
	var current *ServerCandidate
	for i := range candidates {
		if candidates[i].ID == currentID {
			current = &candidates[i]
			break
		}
	}
	if current == nil {
		return FailoverDecision{}, errors.New("current server is missing from candidates")
	}

	if c.currentID != currentID {
		c.currentID = currentID
		c.failureStreak = 0
	}
	bad := current.Status != engine.StatusHealthy || current.Latency < 0 || current.Latency > c.cfg.MaxLatency
	if bad {
		c.failureStreak++
	} else {
		c.failureStreak = 0
	}

	reason := "current_healthy"
	if current.Status != engine.StatusHealthy || current.Latency < 0 {
		reason = "current_unavailable"
	} else if current.Latency > c.cfg.MaxLatency {
		reason = "latency_threshold_exceeded"
	}
	decision := FailoverDecision{CurrentID: currentID, Reason: reason}
	if !bad || c.failureStreak < c.cfg.FailureThreshold {
		return decision, nil
	}
	if !c.lastSwitch.IsZero() && now.Sub(c.lastSwitch) < c.cfg.Cooldown {
		decision.Reason = "cooldown"
		return decision, nil
	}

	eligible := make([]ServerCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ID == "" || candidate.ID == currentID ||
			candidate.Status != engine.StatusHealthy || candidate.Latency < 0 ||
			candidate.Latency > c.cfg.MaxLatency {
			continue
		}
		eligible = append(eligible, candidate)
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].Latency != eligible[j].Latency {
			return eligible[i].Latency < eligible[j].Latency
		}
		if eligible[i].SubscriptionID != eligible[j].SubscriptionID {
			return eligible[i].SubscriptionID < eligible[j].SubscriptionID
		}
		return eligible[i].ID < eligible[j].ID
	})
	if len(eligible) == 0 {
		decision.Reason = "no_healthy_alternative"
		return decision, nil
	}

	target := eligible[0]
	if current.Status == engine.StatusHealthy && current.Latency >= 0 &&
		current.Latency-target.Latency < c.cfg.MinImprovement {
		decision.Reason = "improvement_below_threshold"
		return decision, nil
	}
	decision.Target = target
	decision.Switch = true
	decision.Reason = reason
	c.lastSwitch = now
	c.currentID = target.ID
	c.failureStreak = 0
	return decision, nil
}
