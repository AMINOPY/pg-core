package fusion

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestAllocatorReservesCapacityAtomically(t *testing.T) {
	a, err := NewAllocator([]Path{
		{ID: "only", Healthy: true, RTT: time.Millisecond, Weight: 1, MaxConcurrent: 2, ActiveFlows: 99},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, release1, err := a.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer release1()
	_, release2, err := a.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer release2()
	if _, _, err := a.Acquire(); !errors.Is(err, ErrNoEligiblePath) {
		t.Fatalf("third acquire error = %v, want ErrNoEligiblePath", err)
	}
	if got := a.Snapshot()[0].ActiveFlows; got != 2 {
		t.Fatalf("active flows = %d, want 2 (external count must be ignored)", got)
	}
}

func TestAllocatorConcurrentAcquiresRespectCapacity(t *testing.T) {
	const capacity = 7
	const workers = 64
	a, err := NewAllocator([]Path{
		{ID: "bounded", Healthy: true, Weight: 1, MaxConcurrent: capacity},
	})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var releases []func()
	var succeeded int
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, release, err := a.Acquire()
			if err != nil {
				if !errors.Is(err, ErrNoEligiblePath) {
					t.Errorf("Acquire error = %v", err)
				}
				return
			}
			mu.Lock()
			succeeded++
			releases = append(releases, release)
			mu.Unlock()
		}()
	}
	wg.Wait()
	if succeeded != capacity {
		t.Fatalf("successful acquires = %d, want %d", succeeded, capacity)
	}
	if got := a.Snapshot()[0].ActiveFlows; got != capacity {
		t.Fatalf("active flows = %d, want %d", got, capacity)
	}
	for _, release := range releases {
		release()
	}
	if got := a.Snapshot()[0].ActiveFlows; got != 0 {
		t.Fatalf("active flows after release = %d, want 0", got)
	}
}

func TestAllocatorReleaseIsIdempotent(t *testing.T) {
	a, err := NewAllocator([]Path{{ID: "a", Healthy: true, Weight: 1}})
	if err != nil {
		t.Fatal(err)
	}
	_, release, err := a.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()
	if got := a.Snapshot()[0].ActiveFlows; got != 0 {
		t.Fatalf("active flows after duplicate release = %d, want 0", got)
	}
}

func TestAllocatorCannotRemovePathWithActiveFlow(t *testing.T) {
	a, err := NewAllocator([]Path{{ID: "a", Healthy: true, Weight: 1}})
	if err != nil {
		t.Fatal(err)
	}
	_, release, err := a.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Update([]Path{{ID: "b", Healthy: true, Weight: 1}}); !errors.Is(err, ErrPathInUse) {
		t.Fatalf("update error = %v, want ErrPathInUse", err)
	}
	release()
	if err := a.Update([]Path{{ID: "b", Healthy: true, Weight: 1}}); err != nil {
		t.Fatalf("update after release: %v", err)
	}
}

func TestAllocatorUpdatePreservesCountsAndCanDisablePath(t *testing.T) {
	a, err := NewAllocator([]Path{
		{ID: "a", Healthy: true, Weight: 1},
		{ID: "b", Healthy: true, Weight: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	path, release, err := a.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	updated := []Path{
		{ID: "a", Healthy: false, Weight: 1},
		{ID: "b", Healthy: true, Weight: 1},
	}
	if err := a.Update(updated); err != nil {
		t.Fatal(err)
	}
	var active int
	for _, snapshot := range a.Snapshot() {
		active += snapshot.ActiveFlows
	}
	if active != 1 {
		t.Fatalf("active flow count after update = %d, want 1 (selected %s)", active, path.ID)
	}
	if got, _, err := a.Acquire(); err != nil || got.ID != "b" {
		t.Fatalf("acquire after path disable = %q, %v; want b", got.ID, err)
	}
}
