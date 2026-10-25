package native

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestValidateHealthURL(t *testing.T) {
	for _, raw := range []string{
		"https://example.com/health",
		"http://127.0.0.1:8080/health",
		"",
	} {
		if err := validateHealthURL(raw); err != nil {
			t.Errorf("validateHealthURL(%q): %v", raw, err)
		}
	}
	for _, raw := range []string{
		"example.com/health",
		"http://example.com/health",
		"https://user:secret@example.com/health",
		"file:///etc/passwd",
		"https://example.com:99999/health",
	} {
		if err := validateHealthURL(raw); err == nil {
			t.Errorf("validateHealthURL(%q) unexpectedly succeeded", raw)
		}
	}
}

type limitedWriter struct {
	max int
	got []byte
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if len(p) > w.max {
		p = p[:w.max]
	}
	w.got = append(w.got, p...)
	return len(p), nil
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

func TestWriteAllHandlesShortWrites(t *testing.T) {
	w := &limitedWriter{max: 2}
	want := []byte("partial writes are valid")
	if err := writeAll(w, want); err != nil {
		t.Fatalf("writeAll: %v", err)
	}
	if string(w.got) != string(want) {
		t.Fatalf("written %q, want %q", w.got, want)
	}
}

func TestWriteAllRejectsZeroProgress(t *testing.T) {
	if err := writeAll(zeroWriter{}, []byte("x")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("writeAll error = %v, want io.ErrShortWrite", err)
	}
}

func TestSocks5ConnectHonorsCancellationWithoutDeadline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	greetingRead := make(chan struct{})
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var greeting [3]byte
		if _, err := io.ReadFull(conn, greeting[:]); err != nil {
			return
		}
		close(greetingRead)
		<-ctx.Done()
	}()

	result := make(chan error, 1)
	go func() {
		conn, err := socks5Connect(ctx, ln.Addr().String(), "example.com:443")
		if conn != nil {
			_ = conn.Close()
		}
		result <- err
	}()

	select {
	case <-greetingRead:
	case <-time.After(2 * time.Second):
		t.Fatal("client did not send SOCKS5 greeting")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("socks5Connect error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SOCKS5 handshake did not stop after context cancellation")
	}
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("test proxy did not shut down")
	}
}
