package netrecovery

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeBackend struct {
	state      []byte
	captureErr error
	applyErr   error
	restoreErr error
	verifyErr  error
	captures   int
	applies    int
	restores   int
	verifies   int
}

func (b *fakeBackend) Capture(context.Context) (Snapshot, error) {
	b.captures++
	if b.captureErr != nil {
		return Snapshot{}, b.captureErr
	}
	return Snapshot{Data: append([]byte(nil), b.state...)}, nil
}

func (b *fakeBackend) Apply(_ context.Context, desired DesiredState) error {
	b.applies++
	if b.applyErr != nil {
		return b.applyErr
	}
	b.state = append([]byte(nil), desired.Data...)
	return nil
}

func (b *fakeBackend) Restore(_ context.Context, snapshot Snapshot) error {
	b.restores++
	if b.restoreErr != nil {
		return b.restoreErr
	}
	b.state = append([]byte(nil), snapshot.Data...)
	return nil
}

func (b *fakeBackend) Verify(_ context.Context, snapshot Snapshot) error {
	b.verifies++
	if b.verifyErr != nil {
		return b.verifyErr
	}
	if !reflect.DeepEqual(b.state, snapshot.Data) {
		return errors.New("state does not match captured snapshot")
	}
	return nil
}

func TestSessionStartStopRestoresPriorNetworkState(t *testing.T) {
	backend := &fakeBackend{state: []byte("original routes+dns")}
	session, err := New(backend)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Start(context.Background(), DesiredState{Data: []byte("tunnel routes+dns")}); err != nil {
		t.Fatal(err)
	}
	if !session.Active() {
		t.Fatal("session must remain active while tunnel state is applied")
	}
	if string(backend.state) != "tunnel routes+dns" {
		t.Fatalf("state after start = %q", backend.state)
	}
	if err := session.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if session.Active() {
		t.Fatal("successful restoration must release snapshot")
	}
	if string(backend.state) != "original routes+dns" {
		t.Fatalf("state after stop = %q", backend.state)
	}
	if backend.captures != 1 || backend.restores != 1 || backend.verifies != 1 {
		t.Fatalf("unexpected backend call counts: %+v", backend)
	}
}

func TestSessionRollsBackPartialApplyFailure(t *testing.T) {
	applyErr := errors.New("apply route failed after DNS changed")
	backend := &fakeBackend{state: []byte("original"), applyErr: applyErr}
	session, _ := New(backend)
	err := session.Start(context.Background(), DesiredState{Data: []byte("partial")})
	if !errors.Is(err, applyErr) {
		t.Fatalf("Start error = %v, want apply error", err)
	}
	if session.Active() {
		t.Fatal("successful rollback should release snapshot")
	}
	if string(backend.state) != "original" {
		t.Fatalf("network state not restored: %q", backend.state)
	}
}

func TestSessionRetainsSnapshotWhenRestoreFailsAndAllowsRetry(t *testing.T) {
	backend := &fakeBackend{state: []byte("original")}
	session, _ := New(backend)
	if err := session.Start(context.Background(), DesiredState{Data: []byte("tunnel")}); err != nil {
		t.Fatal(err)
	}
	restoreErr := errors.New("route command temporarily failed")
	backend.restoreErr = restoreErr
	if err := session.Stop(context.Background()); !errors.Is(err, restoreErr) {
		t.Fatalf("Stop error = %v, want restore error", err)
	}
	if !session.Active() {
		t.Fatal("snapshot must be retained after restore failure")
	}
	backend.restoreErr = nil
	if err := session.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if session.Active() || string(backend.state) != "original" {
		t.Fatalf("retry did not restore original state: active=%v state=%q", session.Active(), backend.state)
	}
}

func TestSessionRetainsSnapshotWhenVerificationFails(t *testing.T) {
	backend := &fakeBackend{state: []byte("original")}
	session, _ := New(backend)
	if err := session.Start(context.Background(), DesiredState{Data: []byte("tunnel")}); err != nil {
		t.Fatal(err)
	}
	verifyErr := errors.New("DNS state differs")
	backend.verifyErr = verifyErr
	if err := session.Stop(context.Background()); !errors.Is(err, verifyErr) {
		t.Fatalf("Stop error = %v, want verification error", err)
	}
	if !session.Active() {
		t.Fatal("snapshot must remain available until verification succeeds")
	}
	backend.verifyErr = nil
	if err := session.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSessionRollbackFailureIsRecoverable(t *testing.T) {
	backend := &fakeBackend{state: []byte("original"), applyErr: errors.New("apply failed"), restoreErr: errors.New("restore failed")}
	session, _ := New(backend)
	if err := session.Start(context.Background(), DesiredState{Data: []byte("partial")}); err == nil {
		t.Fatal("expected combined apply/rollback error")
	}
	if !session.Active() {
		t.Fatal("snapshot must remain when rollback failed")
	}
	backend.restoreErr = nil
	if err := session.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if string(backend.state) != "original" {
		t.Fatalf("state after recovery = %q", backend.state)
	}
}

func TestSessionRejectsEmptySnapshotAndDuplicateStart(t *testing.T) {
	backend := &fakeBackend{}
	session, _ := New(backend)
	if err := session.Start(context.Background(), DesiredState{Data: []byte("tunnel")}); !errors.Is(err, ErrSnapshotEmpty) {
		t.Fatalf("Start error = %v, want ErrSnapshotEmpty", err)
	}
	backend.state = []byte("original")
	if err := session.Start(context.Background(), DesiredState{Data: []byte("tunnel")}); err != nil {
		t.Fatal(err)
	}
	if err := session.Start(context.Background(), DesiredState{Data: []byte("second")}); !errors.Is(err, ErrAlreadyActive) {
		t.Fatalf("second Start error = %v, want ErrAlreadyActive", err)
	}
}

func TestSessionRecoverRestoresAndReturnsEngineFailure(t *testing.T) {
	backend := &fakeBackend{state: []byte("original")}
	session, _ := New(backend)
	if err := session.Start(context.Background(), DesiredState{Data: []byte("tunnel")}); err != nil {
		t.Fatal(err)
	}
	engineErr := errors.New("engine exited")
	if err := session.Recover(context.Background(), engineErr); !errors.Is(err, engineErr) {
		t.Fatalf("Recover error = %v, want engine failure", err)
	}
	if session.Active() || string(backend.state) != "original" {
		t.Fatalf("failure recovery did not restore state: active=%v state=%q", session.Active(), backend.state)
	}
}
