package fusion

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestSelectPrefersHealthyLowerCostPath(t *testing.T) {
	got, err := Select([]Path{
		{ID: "slow", Healthy: true, RTT: 120 * time.Millisecond, Weight: 1},
		{ID: "fast", Healthy: true, RTT: 20 * time.Millisecond, Weight: 1},
		{ID: "down", Healthy: false, RTT: time.Millisecond, Weight: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "fast" {
		t.Fatalf("selected %q, want fast", got.ID)
	}
}

func TestSelectAccountsForWeightLossAndLoad(t *testing.T) {
	got, err := Select([]Path{
		{ID: "loaded", Healthy: true, RTT: 10 * time.Millisecond, Weight: 1, ActiveFlows: 8},
		{ID: "weighted", Healthy: true, RTT: 30 * time.Millisecond, Weight: 4},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "weighted" {
		t.Fatalf("selected %q, want weighted", got.ID)
	}

	got, err = Select([]Path{
		{ID: "lossy", Healthy: true, RTT: time.Millisecond, Loss: 0.2, Weight: 1},
		{ID: "clean", Healthy: true, RTT: 30 * time.Millisecond, Weight: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "clean" {
		t.Fatalf("selected %q, want clean path despite its higher RTT", got.ID)
	}
}

func TestSelectSkipsFullAndUnhealthyPaths(t *testing.T) {
	got, err := Select([]Path{
		{ID: "full", Healthy: true, RTT: time.Millisecond, Weight: 1, ActiveFlows: 2, MaxConcurrent: 2},
		{ID: "down", Healthy: false, RTT: time.Millisecond, Weight: 1},
		{ID: "available", Healthy: true, RTT: 50 * time.Millisecond, Weight: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "available" {
		t.Fatalf("selected %q, want available", got.ID)
	}
}

func TestSelectNoEligiblePath(t *testing.T) {
	_, err := Select([]Path{{ID: "down", Healthy: false, Weight: 1}})
	if !errors.Is(err, ErrNoEligiblePath) {
		t.Fatalf("error = %v, want ErrNoEligiblePath", err)
	}
}

func TestSelectRejectsInvalidAndDuplicatePaths(t *testing.T) {
	cases := []struct {
		name  string
		paths []Path
	}{
		{name: "zero weight", paths: []Path{{ID: "a", Weight: 0}}},
		{name: "negative RTT", paths: []Path{{ID: "a", Weight: 1, RTT: -1}}},
		{name: "NaN loss", paths: []Path{{ID: "a", Weight: 1, Loss: math.NaN()}}},
		{name: "loss above one", paths: []Path{{ID: "a", Weight: 1, Loss: 1.1}}},
		{name: "duplicate ID", paths: []Path{{ID: "a", Weight: 1}, {ID: "a", Weight: 1}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Select(tc.paths); !errors.Is(err, ErrInvalidPaths) {
				t.Fatalf("error = %v, want ErrInvalidPaths", err)
			}
		})
	}
}

func TestSelectIsDeterministicOnTies(t *testing.T) {
	paths := []Path{
		{ID: "b", Healthy: true, RTT: 10 * time.Millisecond, Weight: 1},
		{ID: "a", Healthy: true, RTT: 10 * time.Millisecond, Weight: 1},
	}
	for i := 0; i < 5; i++ {
		got, err := Select(paths)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != "a" {
			t.Fatalf("selected %q, want deterministic tie-break to a", got.ID)
		}
	}
}
