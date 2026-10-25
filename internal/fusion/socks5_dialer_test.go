package fusion

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestSOCKS5DialerEncodesDomainIPv4AndIPv6Targets(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		wantAT  byte
		wantHost []byte
	}{
		{name: "domain", target: "example.com:443", wantAT: 3, wantHost: []byte("example.com")},
		{name: "IPv4", target: "192.0.2.10:80", wantAT: 1, wantHost: net.ParseIP("192.0.2.10").To4()},
		{name: "IPv6", target: "[2001:db8::1]:8443", wantAT: 4, wantHost: net.ParseIP("2001:db8::1").To16()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requests := make(chan []byte, 1)
			proxy, closeProxy := startTestSOCKS5Proxy(t, 0, []byte("ok"), requests)
			defer closeProxy()
			dialer := SOCKS5Dialer{ProxyAddress: proxy}
			conn, err := dialer.DialContext(context.Background(), "tcp", tc.target)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			var payload [2]byte
			if _, err := io.ReadFull(conn, payload[:]); err != nil {
				t.Fatal(err)
			}
			if string(payload[:]) != "ok" {
				t.Fatalf("payload = %q, want ok", payload[:])
			}
			request := <-requests
			if len(request) < 4 || request[0] != 5 || request[1] != 1 || request[2] != 0 || request[3] != tc.wantAT {
				t.Fatalf("CONNECT request header = %v, want ATYP %d", request[:minInt(len(request), 4)], tc.wantAT)
			}
			offset := 4
			if tc.wantAT == 3 {
				if len(request) < 5 || int(request[4]) != len(tc.wantHost) {
					t.Fatalf("domain length byte mismatch in request %v", request)
				}
				offset = 5
			}
			if len(request) < offset+len(tc.wantHost)+2 {
				t.Fatalf("CONNECT request too short: %v", request)
			}
			for i := range tc.wantHost {
				if request[offset+i] != tc.wantHost[i] {
					t.Fatalf("encoded host bytes = %v, want %v", request[offset:offset+len(tc.wantHost)], tc.wantHost)
				}
			}
			gotPort := binary.BigEndian.Uint16(request[len(request)-2:])
			wantPort, _ := strconv.Atoi(tc.target[stringsLastColon(tc.target)+1:])
			if int(gotPort) != wantPort {
				t.Fatalf("encoded port = %d, want %d", gotPort, wantPort)
			}
		})
	}
}

func TestSOCKS5DialerRejectsUnsupportedMethodAndConnectFailure(t *testing.T) {
	t.Run("unsupported method", func(t *testing.T) {
		proxy, closeProxy := startTestSOCKS5Proxy(t, 255, nil, nil)
		defer closeProxy()
		_, err := (SOCKS5Dialer{ProxyAddress: proxy}).DialContext(context.Background(), "tcp", "example.com:443")
		if err == nil {
			t.Fatal("expected method negotiation failure")
		}
	})
	t.Run("connect rejected", func(t *testing.T) {
		proxy, closeProxy := startTestSOCKS5Proxy(t, 5, nil, nil)
		defer closeProxy()
		_, err := (SOCKS5Dialer{ProxyAddress: proxy}).DialContext(context.Background(), "tcp", "example.com:443")
		if err == nil {
			t.Fatal("expected CONNECT failure")
		}
	})
}

func TestSOCKS5DialerCancellationInterruptsHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var greeting [3]byte
		_, _ = io.ReadFull(conn, greeting[:])
		close(accepted)
		var blocked [2]byte
		_, _ = io.ReadFull(conn, blocked[:])
	}()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := (SOCKS5Dialer{ProxyAddress: listener.Addr().String()}).DialContext(ctx, "tcp", "example.com:443")
		done <- err
	}()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("proxy did not receive greeting")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("DialContext error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("DialContext did not stop after cancellation")
	}
}

func TestSOCKS5DialerRejectsInvalidInputs(t *testing.T) {
	dialer := SOCKS5Dialer{ProxyAddress: "127.0.0.1:1080"}
	if _, err := dialer.DialContext(context.Background(), "udp", "example.com:53"); !errors.Is(err, ErrUnsupportedNetwork) {
		t.Fatalf("network error = %v, want ErrUnsupportedNetwork", err)
	}
	if _, err := dialer.DialContext(context.Background(), "tcp", "bad-address"); err == nil {
		t.Fatal("expected invalid target error")
	}
	if _, err := (SOCKS5Dialer{ProxyAddress: "not-an-address"}).DialContext(context.Background(), "tcp", "example.com:443"); !errors.Is(err, ErrInvalidSOCKS5Proxy) {
		t.Fatalf("proxy address error = %v, want ErrInvalidSOCKS5Proxy", err)
	}
}

func startTestSOCKS5Proxy(t *testing.T, replyCode byte, payload []byte, requests chan<- []byte) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var greeting [3]byte
		if _, err := io.ReadFull(conn, greeting[:]); err != nil {
			return
		}
		if greeting[0] != 5 || greeting[1] != 1 || greeting[2] != 0 {
			return
		}
		if _, err := conn.Write([]byte{5, 0}); err != nil {
			return
		}
		var header [4]byte
		if _, err := io.ReadFull(conn, header[:]); err != nil {
			return
		}
		var addressBytes []byte
		switch header[3] {
		case 1:
			addressBytes = make([]byte, 4)
		case 4:
			addressBytes = make([]byte, 16)
		case 3:
			var length [1]byte
			if _, err := io.ReadFull(conn, length[:]); err != nil {
				return
			}
			addressBytes = make([]byte, int(length[0]))
			addressBytes = append([]byte{length[0]}, addressBytes...)
		default:
			return
		}
		if header[3] == 3 {
			if _, err := io.ReadFull(conn, addressBytes[1:]); err != nil {
				return
			}
		} else if _, err := io.ReadFull(conn, addressBytes); err != nil {
			return
		}
		var port [2]byte
		if _, err := io.ReadFull(conn, port[:]); err != nil {
			return
		}
		if requests != nil {
			request := append([]byte(nil), header[:]...)
			request = append(request, addressBytes...)
			request = append(request, port[:]...)
			requests <- request
		}
		if _, err := conn.Write([]byte{5, replyCode, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil || replyCode != 0 {
			return
		}
		if len(payload) > 0 {
			_, _ = conn.Write(payload)
		}
	}()
	return listener.Addr().String(), func() {
		_ = listener.Close()
		<-done
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func stringsLastColon(value string) int {
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] == ':' {
			return i
		}
	}
	return -1
}
