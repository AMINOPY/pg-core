package mobile

import (
	"errors"
	"syscall"
	"testing"
)

type fakeRawConn struct {
	fd uintptr
}

func (f fakeRawConn) Control(fn func(uintptr)) error { fn(f.fd); return nil }
func (f fakeRawConn) Read(fn func(uintptr) bool) error {
	if !fn(f.fd) {
		return errors.New("read callback declined")
	}
	return nil
}
func (f fakeRawConn) Write(fn func(uintptr) bool) error {
	if !fn(f.fd) {
		return errors.New("write callback declined")
	}
	return nil
}

type fakeProtector struct {
	fd   int
	ok   bool
	calls int
}

func (p *fakeProtector) Protect(fd int) bool {
	p.fd = fd
	p.calls++
	return p.ok
}

var _ syscall.RawConn = fakeRawConn{}

func TestSocketControlProtectsBeforeDialContinues(t *testing.T) {
	protector := &fakeProtector{ok: true}
	if err := (socketControl{protector: protector}).control("tcp", "203.0.113.10:443", fakeRawConn{fd: 41}); err != nil {
		t.Fatalf("control returned error: %v", err)
	}
	if protector.calls != 1 || protector.fd != 41 {
		t.Fatalf("Protect called %d times with fd %d; want once with fd 41", protector.calls, protector.fd)
	}
}

func TestSocketControlFailsClosedWhenProtectionRejected(t *testing.T) {
	protector := &fakeProtector{ok: false}
	err := (socketControl{protector: protector}).control("tcp", "203.0.113.10:443", fakeRawConn{fd: 42})
	if err == nil {
		t.Fatal("expected protection failure")
	}
	if protector.calls != 1 || protector.fd != 42 {
		t.Fatalf("Protect called %d times with fd %d; want once with fd 42", protector.calls, protector.fd)
	}
}

func TestSocketControlRequiresProtector(t *testing.T) {
	if err := (socketControl{}).control("tcp", "203.0.113.10:443", fakeRawConn{fd: 43}); err == nil {
		t.Fatal("expected missing protector to fail closed")
	}
}

func TestMobileEngineRequiresProtector(t *testing.T) {
	e := NewEngine()
	if err := e.Start(`{"key":"test","server":"127.0.0.1:443"}`, nil); err == nil {
		t.Fatal("expected Start to reject missing protector")
	}
	if e.Addr() != "" {
		t.Fatal("engine must not listen when protector is missing")
	}
}
