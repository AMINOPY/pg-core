package diagnostics

import (
	"context"
	"errors"
	"testing"
)

type testChecker struct {
	finding Finding
	err     error
}

func (c testChecker) Check(context.Context) (Finding, error) { return c.finding, c.err }

type testRepairer struct {
	calls int
	err   error
}

func (r *testRepairer) Repair(context.Context, Finding) error { r.calls++; return r.err }

type testVerifier struct {
	calls int
	err   error
}

func (v *testVerifier) Verify(context.Context, Finding) error { v.calls++; return v.err }

func TestRunOnceRequiresVerifiedRepair(t *testing.T) {
	rep := &testRepairer{}
	ver := &testVerifier{}
	r := Runner{
		Checker:     testChecker{finding: Finding{Code: "worker_unhealthy", Component: "engine-a", Recoverable: true}},
		Repairer:    rep,
		Verifier:    ver,
		MaxAttempts: 1,
	}
	got := r.RunOnce(context.Background())
	if !got.Repaired || !got.Attempted || rep.calls != 1 || ver.calls != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestRunOnceDoesNotClaimSuccessWithoutVerifier(t *testing.T) {
	rep := &testRepairer{}
	r := Runner{Checker: testChecker{finding: Finding{Code: "restart", Recoverable: true}}, Repairer: rep}
	got := r.RunOnce(context.Background())
	if got.Repaired || got.Err == nil || rep.calls != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestRunOnceBoundsAttempts(t *testing.T) {
	rep := &testRepairer{err: errors.New("restart failed")}
	r := Runner{Checker: testChecker{finding: Finding{Code: "restart", Recoverable: true}}, Repairer: rep, MaxAttempts: 2}
	got := r.RunOnce(context.Background())
	if got.Repaired || rep.calls != 2 || got.Err == nil {
		t.Fatalf("unexpected result: %+v calls=%d", got, rep.calls)
	}
}

func TestRunOnceDoesNotRepairNonRecoverableFinding(t *testing.T) {
	rep := &testRepairer{}
	r := Runner{Checker: testChecker{finding: Finding{Code: "config_invalid", Recoverable: false}}, Repairer: rep}
	got := r.RunOnce(context.Background())
	if got.Attempted || rep.calls != 0 || got.Err != nil {
		t.Fatalf("unexpected result: %+v", got)
	}
}
