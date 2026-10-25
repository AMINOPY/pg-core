package socksprobe

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
)

func TestValidateTarget(t *testing.T) {
	for _, target := range []string{"example.com:443", "127.0.0.1:80", "[::1]:443"} {
		if err := ValidateTarget(target); err != nil {
			t.Errorf("valid target %q rejected: %v", target, err)
		}
	}
	for _, target := range []string{"", "example.com", "example.com:0", "example.com:65536", "example.com:443\n"} {
		if err := ValidateTarget(target); err == nil {
			t.Errorf("invalid target %q accepted", target)
		}
	}
}

func TestProbeTCPPerformsSOCKSConnect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		greeting := make([]byte, 3)
		if _, err := io.ReadFull(conn, greeting); err != nil {
			serverErr <- err
			return
		}
		if greeting[0] != 5 || greeting[1] != 1 || greeting[2] != 0 {
			serverErr <- errors.New("unexpected SOCKS greeting")
			return
		}
		if _, err := conn.Write([]byte{5, 0}); err != nil {
			serverErr <- err
			return
		}
		head := make([]byte, 4)
		if _, err := io.ReadFull(conn, head); err != nil {
			serverErr <- err
			return
		}
		if head[0] != 5 || head[1] != 1 || head[3] != 3 {
			serverErr <- errors.New("unexpected SOCKS CONNECT request")
			return
		}
		length := make([]byte, 1)
		if _, err := io.ReadFull(conn, length); err != nil {
			serverErr <- err
			return
		}
		nameAndPort := make([]byte, int(length[0])+2)
		if _, err := io.ReadFull(conn, nameAndPort); err != nil {
			serverErr <- err
			return
		}
		if string(nameAndPort[:len(nameAndPort)-2]) != "example.com" {
			serverErr <- errors.New("unexpected SOCKS target")
			return
		}
		_, err = conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
		serverErr <- err
	}()

	if err := ProbeTCP(context.Background(), listener.Addr().String(), "example.com:443"); err != nil {
		t.Fatalf("ProbeTCP failed: %v", err)
	}
	if err := <-serverErr; err != nil {
		t.Fatalf("SOCKS test server failed: %v", err)
	}
}

func TestProbeTCPRejectsNilAndCancelledContext(t *testing.T) {
	if err := ProbeTCP(nil, "127.0.0.1:1", "example.com:443"); err == nil {
		t.Fatal("expected nil context to be rejected")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ProbeTCP(ctx, "127.0.0.1:1", "example.com:443"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
