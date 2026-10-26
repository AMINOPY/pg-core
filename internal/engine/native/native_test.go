package native

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AMINOPY/pg-core/internal/engine"
	"github.com/AMINOPY/pg-core/internal/fronting"
)

type env struct {
	srv      *httptest.Server
	port     int
	pool     *x509.CertPool
	echoAddr string
}

// newEnv starts a TLS tunnel server (cert valid for example.com) and a TCP echo server.
func newEnv(t testing.TB) *env {
	t.Helper()
	srv := httptest.NewTLSServer(&Server{Path: "/secret"})
	t.Cleanup(srv.Close)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	_, p, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(p)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); io.Copy(c, c) }()
		}
	}()
	return &env{srv: srv, port: port, pool: pool, echoAddr: ln.Addr().String()}
}

func (e *env) dialer(ips ...string) *Dialer {
	return &Dialer{
		SNI: "example.com", EdgeIPs: ips, Port: e.port, Path: "/secret",
		Timeout:   2 * time.Second,
		TLSConfig: &tls.Config{RootCAs: e.pool},
	}
}

func roundTrip(t *testing.T, c io.ReadWriter, msg string) {
	t.Helper()
	if _, err := c.Write([]byte(msg)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != msg {
		t.Fatalf("echo: %q err=%v", buf, err)
	}
}

func TestTunnelThroughEdgeIPWithSNI(t *testing.T) {
	e := newEnv(t)
	host, port, _ := net.SplitHostPort(e.echoAddr)
	p, _ := strconv.Atoi(port)
	c, err := e.dialer("127.0.0.1").DialTarget(context.Background(), host, p)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	roundTrip(t, c, "hello through the tunnel")
	roundTrip(t, c, strings.Repeat("x", 200000)) // multi-frame payload
}

func TestDeadEdgeIPIsSkipped(t *testing.T) {
	e := newEnv(t)
	host, port, _ := net.SplitHostPort(e.echoAddr)
	p, _ := strconv.Atoi(port)
	d := e.dialer("127.0.0.2", "127.0.0.1") // first one has no listener
	c, err := d.DialTarget(context.Background(), host, p)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	roundTrip(t, c, "ping")
	if d.last != 1 {
		t.Fatalf("working IP not remembered: %d", d.last)
	}
}

func TestWrongKeyAndWrongSNIFail(t *testing.T) {
	e := newEnv(t)
	d := e.dialer("127.0.0.1")
	d.Path = "/wrong"
	if err := d.Probe(context.Background()); err == nil {
		t.Fatal("wrong key accepted")
	}
	d = e.dialer("127.0.0.1")
	d.SNI = "other.example.org" // certificate does not match: must be rejected
	if err := d.Probe(context.Background()); err == nil {
		t.Fatal("certificate mismatch accepted")
	}
}

func TestSOCKS5EndToEnd(t *testing.T) {
	e := newEnv(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go serveSOCKS5(context.Background(), ln, e.dialer("127.0.0.1"))

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Write([]byte{5, 1, 0})
	var r [2]byte
	io.ReadFull(c, r[:])
	host, port, _ := net.SplitHostPort(e.echoAddr)
	pn, _ := strconv.Atoi(port)
	req := append([]byte{5, 1, 0, 1}, net.ParseIP(host).To4()...)
	req = append(req, byte(pn>>8), byte(pn))
	c.Write(req)
	var rep [10]byte
	if _, err := io.ReadFull(c, rep[:]); err != nil || rep[1] != 0 {
		t.Fatalf("socks reply %v err=%v", rep, err)
	}
	roundTrip(t, c, "via socks5")
}

func TestAdapterLifecycleAndHealth(t *testing.T) {
	e := newEnv(t)
	a := New()
	a.Fronting = &fronting.Settings{SNI: "example.com", EdgeIPs: []string{"127.0.0.1"}}
	// Trust the test certificate through the dialer created by the adapter.
	prof, _ := json.Marshal(Profile{Key: "secret", Port: e.port, Listen: "127.0.0.1:0", TimeoutMS: 2000})
	ctx := context.Background()
	if err := a.ValidateConfig(ctx, prof); err != nil {
		t.Fatal(err)
	}
	if err := a.Start(ctx, prof); err != nil {
		t.Fatal(err)
	}
	defer a.Stop(ctx)
	if a.Addr() == "" {
		t.Fatal("no listen address")
	}
	// The system roots do not contain the test CA, so the probe must report degraded.
	rep, err := a.Health(ctx)
	if err != nil || rep.Status != engine.StatusDegraded {
		t.Fatalf("health: %+v %v", rep, err)
	}
	a.run.dialer.TLSConfig = &tls.Config{RootCAs: e.pool}
	rep, err = a.Health(ctx)
	if err != nil || rep.Status != engine.StatusHealthy {
		t.Fatalf("health: %+v %v", rep, err)
	}
	if err := a.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	rep, _ = a.Health(ctx)
	if rep.Status != engine.StatusStopped {
		t.Fatalf("after stop: %+v", rep)
	}
}

func TestAdapterRejectsNilAndCancelledContexts(t *testing.T) {
	a := New()
	profile := []byte(`{"key":"k","server":"edge.example.com:443"}`)
	if err := a.ValidateConfig(nil, profile); err == nil {
		t.Fatal("ValidateConfig should reject nil context")
	}
	if err := a.Start(nil, profile); err == nil {
		t.Fatal("Start should reject nil context")
	}
	if _, err := a.Health(nil); err == nil {
		t.Fatal("Health should reject nil context")
	}
	if err := a.Stop(nil); err == nil {
		t.Fatal("Stop should reject nil context")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.ValidateConfig(ctx, profile); err != context.Canceled {
		t.Fatalf("ValidateConfig error = %v, want context.Canceled", err)
	}
	if err := a.Start(ctx, profile); err != context.Canceled {
		t.Fatalf("Start error = %v, want context.Canceled", err)
	}
	if _, err := a.Health(ctx); err != context.Canceled {
		t.Fatalf("Health error = %v, want context.Canceled", err)
	}
	if a.Addr() != "" {
		t.Fatal("cancelled Start must not open a listener")
	}
}

func TestProfileValidation(t *testing.T) {
	a := New()
	for _, bad := range []string{
		`{}`,
		`{"key":"k","listen":"0.0.0.0:1080"}`,
		`{"key":"k","unknown":1}`,
		`{"key":"k"}`, // no fronting and no server
	} {
		if err := a.ValidateConfig(context.Background(), []byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	if err := a.ValidateConfig(context.Background(), []byte(`{"key":"k","server":"edge.example.com:8443"}`)); err != nil {
		t.Fatal(err)
	}
}

// BenchmarkWebSocketTunnelWrite measures the WebSocket framing and masking
// overhead on a local in-memory full-duplex connection. It is a regression
// baseline, not a substitute for real network throughput measurements.
func BenchmarkWebSocketTunnelWrite(b *testing.B) {
	clientConn, serverConn := net.Pipe()
	client := newWS(clientConn, nil, true)
	server := newWS(serverConn, nil, false)
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, server)
		close(done)
	}()
	payload := make([]byte, 256<<10)
	for i := range payload {
		payload[i] = byte(i)
	}
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.Write(payload); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	_ = client.Close()
	_ = serverConn.Close()
	<-done
}

func TestSOCKS5RejectsUnofferedAuthenticationMethod(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	done := make(chan error, 1)
	go func() {
		_, _, err := socksHandshake(server)
		done <- err
	}()
	if _, err := client.Write([]byte{5, 1, 2}); err != nil {
		t.Fatal(err)
	}
	var reply [2]byte
	if _, err := io.ReadFull(client, reply[:]); err != nil {
		t.Fatal(err)
	}
	if reply != [2]byte{5, 0xff} {
		t.Fatalf("method reply = %v, want [5 255]", reply)
	}
	if err := <-done; err == nil {
		t.Fatal("handshake accepted a client that did not offer no-auth")
	}
}

func TestSOCKS5RejectsMalformedRequestFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  []byte
	}{
		{name: "nonzero reserved byte", req: []byte{5, 1, 1, 1, 127, 0, 0, 1, 0, 80}},
		{name: "zero port", req: []byte{5, 1, 0, 1, 127, 0, 0, 1, 0, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, client := net.Pipe()
			defer server.Close()
			defer client.Close()

			done := make(chan error, 1)
			go func() {
				_, _, err := socksHandshake(server)
				done <- err
			}()
			if _, err := client.Write([]byte{5, 1, 0}); err != nil {
				t.Fatal(err)
			}
			var reply [2]byte
			if _, err := io.ReadFull(client, reply[:]); err != nil {
				t.Fatal(err)
			}
			if reply != [2]byte{5, 0} {
				t.Fatalf("method reply = %v, want [5 0]", reply)
			}
			// net.Pipe writes block until the peer reads every byte. The
			// server intentionally returns after the malformed four-byte header,
			// so send asynchronously to avoid blocking the test on trailing bytes.
			go func() { _, _ = client.Write(tc.req) }()
			if err := <-done; err == nil {
				t.Fatal("accepted malformed SOCKS5 request")
			}
		})
	}
}

func TestStopWaitsForActiveSOCKSSessions(t *testing.T) {
	a := New()
	profile := []byte(`{"key":"secret","server":"127.0.0.1:443","listen":"127.0.0.1:0"}`)
	if err := a.Start(context.Background(), profile); err != nil {
		t.Fatal(err)
	}

	// A client that stalls mid-handshake must be closed when Stop cancels the
	// engine, and Stop must not return while the handler still owns resources.
	client, err := net.Dial("tcp", a.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Write([]byte{5, 1, 0}); err != nil {
		t.Fatal(err)
	}
	var reply [2]byte
	if _, err := io.ReadFull(client, reply[:]); err != nil {
		t.Fatal(err)
	}
	if reply != [2]byte{5, 0} {
		t.Fatalf("SOCKS greeting reply = %v", reply)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := a.Stop(ctx); err != nil {
		t.Fatalf("Stop did not finish cleanup: %v", err)
	}
	if a.Addr() != "" {
		t.Fatal("engine still reports a listener after Stop")
	}
}


// BenchmarkNativeTunnelRoundTrip measures payload throughput over the complete
// native TLS/WebSocket tunnel to a local TCP echo target. It is a local
// regression baseline, not a claim about Internet or mobile-network speed.
func BenchmarkNativeTunnelRoundTrip(b *testing.B) {
	e := newEnv(b)
	host, portText, err := net.SplitHostPort(e.echoAddr)
	if err != nil {
		b.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		b.Fatal(err)
	}
	conn, err := e.dialer("127.0.0.1").DialTarget(context.Background(), host, port)
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()

	payload := make([]byte, 64<<10)
	for i := range payload {
		payload[i] = byte(i)
	}
	reply := make([]byte, len(payload))
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := conn.Write(payload); err != nil {
			b.Fatal(err)
		}
		if _, err := io.ReadFull(conn, reply); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
}
