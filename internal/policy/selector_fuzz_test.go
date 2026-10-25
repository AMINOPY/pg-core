package policy

import (
	"errors"
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
)

func FuzzSelectDeterministicAndEligible(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Add([]byte{})
	f.Add([]byte{255, 255, 0, 0, 127, 1, 42})

	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 512 {
			raw = raw[:512]
		}
		candidates := make([]Candidate, 0, len(raw)/3)
		statuses := [...]engine.Status{
			engine.StatusStopped,
			engine.StatusStarting,
			engine.StatusHealthy,
			engine.StatusDegraded,
			engine.StatusFailed,
		}
		for i := 0; i+2 < len(raw); i += 3 {
			id := string([]byte{'a' + raw[i]%26})
			candidates = append(candidates, Candidate{
				ID:      id,
				Status:  statuses[raw[i+1]%byte(len(statuses))],
				Latency: time.Duration(raw[i+2]) * time.Millisecond,
			})
		}

		got, err := Select(candidates)
		if errors.Is(err, ErrNoHealthyCandidate) {
			for _, c := range candidates {
				if c.ID != "" && c.Latency >= 0 &&
					(c.Status == engine.StatusHealthy || c.Status == engine.StatusDegraded) {
					t.Fatalf("eligible candidate %q exists but Select returned no candidate", c.ID)
				}
			}
			return
		}
		if err != nil {
			t.Fatalf("unexpected Select error: %v", err)
		}
		if got.ID == "" || got.Latency < 0 ||
			(got.Status != engine.StatusHealthy && got.Status != engine.StatusDegraded) {
			t.Fatalf("Select returned ineligible candidate: %+v", got)
		}

		reversed := append([]Candidate(nil), candidates...)
		for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
			reversed[left], reversed[right] = reversed[right], reversed[left]
		}
		again, err := Select(reversed)
		if err != nil {
			t.Fatalf("reversed input unexpectedly failed: %v", err)
		}
		if got != again {
			t.Fatalf("selection depends on input order: got %+v, reversed got %+v", got, again)
		}
	})
}
