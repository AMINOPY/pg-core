// Package engine defines the boundary between the coordinator and independent
// network engines. It contains no protocol or traffic-obfuscation implementation.
package engine

import (
	"context"
	"errors"
	"time"
)

type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusFailed   Status = "failed"
)

type Capabilities struct {
	TUN          bool `json:"tun"`
	DNS          bool `json:"dns"`
	Routing      bool `json:"routing"`
	HealthChecks bool `json:"health_checks"`
}

type HealthReport struct {
	EngineID  string        `json:"engine_id"`
	Status    Status        `json:"status"`
	Latency   time.Duration `json:"latency_ns,omitempty"`
	CheckedAt time.Time     `json:"checked_at"`
	ErrorCode string        `json:"error_code,omitempty"`
}

// Adapter implementations must honor cancellation and never log credentials
// or raw secret-bearing configuration.
type Adapter interface {
	ID() string
	Capabilities() Capabilities
	ValidateConfig(context.Context, []byte) error
	Start(context.Context, []byte) error
	Health(context.Context) (HealthReport, error)
	Stop(context.Context) error
}

var ErrNotImplemented = errors.New("engine adapter is not implemented")
