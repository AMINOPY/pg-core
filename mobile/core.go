// Package mobile exposes a stable, JSON-only API suitable for gomobile bindings.
//
// The API deliberately contains no platform-specific VPN code: Android owns
// the Flutter V2Ray session and must report verified switch outcomes here.
package mobile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
	"github.com/AMINOPY/pg-core/internal/policy"
)

const (
	maxConfigBytes = 16 << 10
	maxSampleBytes = 256 << 10
	maxCandidates  = 64
	maxLatencyMS   = 7 * 24 * 60 * 60 * 1000
)

var ErrInvalidInput = errors.New("invalid mobile bridge input")

type config struct {
	FailureThreshold int   `json:"failure_threshold"`
	MaxLatencyMS     int64 `json:"max_latency_ms"`
	CooldownMS       int64 `json:"cooldown_ms"`
	MinImprovementMS int64 `json:"min_improvement_ms"`
}

type candidateInput struct {
	ID             string `json:"id"`
	SubscriptionID string `json:"subscription_id"`
	Status         string `json:"status"`
	LatencyMS      int64  `json:"latency_ms"`
}

type sampleInput struct {
	CurrentID  string            `json:"current_id"`
	Candidates []candidateInput   `json:"candidates"`
}

type decisionOutput struct {
	CurrentID string          `json:"current_id"`
	Switch    bool            `json:"switch"`
	TargetID  string          `json:"target_id,omitempty"`
	Reason    string          `json:"reason"`
	LatencyMS int64           `json:"target_latency_ms,omitempty"`
}

type Core struct {
	mu         sync.Mutex
	controller *policy.FailoverController
	now        func() time.Time
}

// NewCore creates a thread-safe policy facade. All duration values are in ms.
func NewCore(configJSON string) (*Core, error) {
	if len(configJSON) == 0 || len(configJSON) > maxConfigBytes {
		return nil, ErrInvalidInput
	}
	var cfg config
	if err := decodeStrict([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("%w: config: %v", ErrInvalidInput, err)
	}
	if cfg.MaxLatencyMS <= 0 || cfg.MaxLatencyMS > maxLatencyMS ||
		cfg.CooldownMS < 0 || cfg.CooldownMS > maxLatencyMS ||
		cfg.MinImprovementMS < 0 || cfg.MinImprovementMS > maxLatencyMS ||
		cfg.FailureThreshold < 1 || cfg.FailureThreshold > 100 {
		return nil, ErrInvalidInput
	}
	controller, err := policy.NewFailoverController(policy.FailoverConfig{
		FailureThreshold: cfg.FailureThreshold,
		MaxLatency:       time.Duration(cfg.MaxLatencyMS) * time.Millisecond,
		Cooldown:         time.Duration(cfg.CooldownMS) * time.Millisecond,
		MinImprovement:   time.Duration(cfg.MinImprovementMS) * time.Millisecond,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return &Core{controller: controller, now: time.Now}, nil
}

// EvaluateJSON consumes bounded, sanitized health samples and returns a JSON
// proposal. A proposal is not an executed switch; call ConfirmSwitch only
// after the platform has switched and independently verified connectivity.
func (c *Core) EvaluateJSON(sampleJSON string) (string, error) {
	if c == nil || c.controller == nil {
		return "", ErrInvalidInput
	}
	if len(sampleJSON) == 0 || len(sampleJSON) > maxSampleBytes {
		return "", ErrInvalidInput
	}
	var sample sampleInput
	if err := decodeStrict([]byte(sampleJSON), &sample); err != nil {
		return "", fmt.Errorf("%w: sample: %v", ErrInvalidInput, err)
	}
	if strings.TrimSpace(sample.CurrentID) == "" || len(sample.CurrentID) > 256 ||
		len(sample.Candidates) == 0 || len(sample.Candidates) > maxCandidates {
		return "", ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(sample.Candidates))
	candidates := make([]policy.ServerCandidate, 0, len(sample.Candidates))
	for _, item := range sample.Candidates {
		if strings.TrimSpace(item.ID) == "" || len(item.ID) > 256 ||
			len(item.SubscriptionID) > 256 || item.LatencyMS < -1 || item.LatencyMS > maxLatencyMS {
			return "", ErrInvalidInput
		}
		if _, exists := seen[item.ID]; exists {
			return "", fmt.Errorf("%w: duplicate candidate ID", ErrInvalidInput)
		}
		seen[item.ID] = struct{}{}
		status := engine.Status(item.Status)
		switch status {
		case engine.StatusHealthy, engine.StatusDegraded, engine.StatusFailed, engine.StatusStopped:
		default:
			return "", fmt.Errorf("%w: unsupported candidate status", ErrInvalidInput)
		}
		candidates = append(candidates, policy.ServerCandidate{
			ID: item.ID, SubscriptionID: item.SubscriptionID,
			Status: status, Latency: time.Duration(item.LatencyMS) * time.Millisecond,
		})
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	decision, err := c.controller.Evaluate(sample.CurrentID, candidates, c.now())
	if err != nil {
		return "", err
	}
	out := decisionOutput{CurrentID: decision.CurrentID, Switch: decision.Switch, Reason: decision.Reason}
	if decision.Switch {
		out.TargetID = decision.Target.ID
		out.LatencyMS = decision.Target.Latency.Milliseconds()
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// ConfirmSwitch must only be called after the platform confirms the new
// session is connected and a separate connectivity check succeeds.
func (c *Core) ConfirmSwitch(targetID string) error {
	if c == nil || c.controller == nil || strings.TrimSpace(targetID) == "" || len(targetID) > 256 {
		return ErrInvalidInput
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.controller.ConfirmSwitch(targetID, c.now())
}

// CancelSwitch abandons the current proposal after a failed/aborted platform switch.
func (c *Core) CancelSwitch() {
	if c == nil || c.controller == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.controller.CancelSwitch()
}

func decodeStrict(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}
