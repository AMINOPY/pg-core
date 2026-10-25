package fusion

import (
	"testing"
)

func TestSelectDoesNotOverflowActiveFlowScore(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	got, err := Select([]Path{
		{ID: "extreme", Healthy: true, Weight: 1, ActiveFlows: maxInt},
		{ID: "idle", Healthy: true, Weight: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "idle" {
		t.Fatalf("selected %q, want idle path over extreme active-flow counter", got.ID)
	}
}
