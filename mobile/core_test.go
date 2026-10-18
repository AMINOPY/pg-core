package mobile

import (
	"encoding/json"
	"strings"
	"testing"
)

func newTestCore(t *testing.T) *Core {
	t.Helper()
	core, err := NewCore(`{"failure_threshold":1,"max_latency_ms":1500,"cooldown_ms":1000,"min_improvement_ms":100}`)
	if err != nil {
		t.Fatal(err)
	}
	return core
}

func TestEvaluateJSONProposesBestHealthyTarget(t *testing.T) {
	core := newTestCore(t)
	input := `{"current_id":"slow","candidates":[{"id":"slow","subscription_id":"sub-b","status":"failed","latency_ms":-1},{"id":"fast","subscription_id":"sub-a","status":"healthy","latency_ms":80}]}`
	raw, err := core.EvaluateJSON(input)
	if err != nil {
		t.Fatal(err)
	}
	var got decisionOutput
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Switch || got.TargetID != "fast" || got.LatencyMS != 80 {
		t.Fatalf("unexpected decision: %+v", got)
	}
	if err := core.ConfirmSwitch("fast"); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateJSONRejectsUnknownFieldsAndDuplicateIDs(t *testing.T) {
	core := newTestCore(t)
	unknown := `{"current_id":"a","unexpected":true,"candidates":[{"id":"a","status":"healthy","latency_ms":10}]}`
	if _, err := core.EvaluateJSON(unknown); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
	duplicate := `{"current_id":"a","candidates":[{"id":"a","status":"healthy","latency_ms":10},{"id":"a","status":"healthy","latency_ms":20}]}`
	if _, err := core.EvaluateJSON(duplicate); err == nil {
		t.Fatal("expected duplicate IDs to be rejected")
	}
}

func TestConfirmRequiresMatchingPendingTarget(t *testing.T) {
	core := newTestCore(t)
	if err := core.ConfirmSwitch("target"); err == nil {
		t.Fatal("expected no-pending-switch error")
	}
	_, err := core.EvaluateJSON(`{"current_id":"slow","candidates":[{"id":"slow","status":"failed","latency_ms":-1},{"id":"fast","status":"healthy","latency_ms":80}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.ConfirmSwitch("wrong"); err == nil {
		t.Fatal("expected mismatched target to be rejected")
	}
	core.CancelSwitch()
	if err := core.ConfirmSwitch("fast"); err == nil {
		t.Fatal("cancelled proposal must not be confirmable")
	}
}

func TestConfigIsStrictAndBounded(t *testing.T) {
	if _, err := NewCore(`{"failure_threshold":1,"max_latency_ms":1000,"extra":1}`); err == nil {
		t.Fatal("expected unknown config field to be rejected")
	}
	if _, err := NewCore(strings.Repeat("x", maxConfigBytes+1)); err == nil {
		t.Fatal("expected oversized config to be rejected")
	}
	if _, err := NewCore(`{"failure_threshold":0,"max_latency_ms":1000}`); err == nil {
		t.Fatal("expected invalid threshold to be rejected")
	}
}

func FuzzNewCoreAndEvaluateJSONNoPanic(f *testing.F) {
	f.Add("")
	f.Add("{")
	f.Add(`{"failure_threshold":1,"max_latency_ms":1000,"cooldown_ms":0,"min_improvement_ms":0}`)
	f.Add(`{"current_id":"a","candidates":[]}`)
	f.Fuzz(func(t *testing.T, raw string) {
		core, err := NewCore(raw)
		if err != nil {
			return
		}
		_, _ = core.EvaluateJSON(raw)
	})
}
