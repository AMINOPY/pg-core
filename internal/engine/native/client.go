package native

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Dialer opens tunnel connections. It connects to the listed edge addresses
// (IPs, no DNS lookup of the SNI) and announces SNI in the TLS ClientHello;
// certificate verification stays on. This is the CDN/SNI feature.
type Dialer struct {
	SNI     string   // TLS server name
	Host    string   // HTTP Host header; defaults to SNI
	EdgeIPs []string // addresses to try; an IP is preferred, a host name is resolved normally
	Port    int      // default 443
	Path    string   // secret path, for example "/s3cr3t"

	Timeout time.Duration
	// Control is called for every outgoing socket. On Android pass a function
	// that calls VpnService.protect(fd) so tunnel traffic does not loop back.
	Control func(network, address string, c syscall.RawConn) error
	// TLSConfig is optional (tests); ServerName and ALPN are always set here.
	TLSConfig *tls.Config

	mu   sync.Mutex
	last int
}

func (d *Dialer) timeout() time.Duration {
	if d.Timeout > 0 {
		return d.Timeout
	}
	return 8 * time.Second
}

// open returns a ready WebSocket to the first edge that completes TLS + upgrade.
func (d *Dialer) open(ctx context.Context) (*wsConn, error) {
	if len(d.EdgeIPs) == 0 || d.SNI == "" {
		return nil, errors.New("SNI and at least one edge address are required")
	}
	port := d.Port
	if port == 0 {
		port = 443
	}
	host := d.Host
	if host == "" {
		host = d.SNI
	}
	d.mu.Lock()
	start := d.last
	d.mu.Unlock()

	var lastErr error
	for i := 0; i < len(d.EdgeIPs); i++ {
		idx := (start + i) % len(d.EdgeIPs)
		ws, err := d.openOne(ctx, d.EdgeIPs[idx], port, host)
		if err == nil {
			d.mu.Lock()
			d.last = idx
			d.mu.Unlock()
			return ws, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, fmt.Errorf("no edge reachable: %w", lastErr)
}

func (d *Dialer) openOne(ctx context.Context, edge string, port int, host string) (*wsConn, error) {
	ctx, cancel := context.WithTimeout(ctx, d.timeout())
	defer cancel()
	nd := net.Dialer{Control: d.Control}
	raw, err := nd.DialContext(ctx, "tcp", net.JoinHostPort(edge, strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	var cfg *tls.Config
	if d.TLSConfig != nil {
		cfg = d.TLSConfig.Clone()
	} else {
		cfg = &tls.Config{}
	}
	cfg.ServerName = d.SNI
	cfg.NextProtos = []string{"http/1.1"}
	if cfg.MinVersion == 0 {
		cfg.MinVersion = tls.VersionTLS12
	}
	tc := tls.Client(raw, cfg)
	if err := tc.HandshakeContext(ctx); err != nil {
		raw.Close()
		return nil, err
	}
	// The handshake deadline is the context; bound the upgrade too.
	_ = tc.SetDeadline(time.Now().Add(d.timeout()))
	ws, err := clientHandshake(tc, host, d.Path)
	if err != nil {
		tc.Close()
		return nil, err
	}
	_ = tc.SetDeadline(time.Time{})
	return ws, nil
}

// Probe checks that an edge completes TLS and the WebSocket upgrade.
func (d *Dialer) Probe(ctx context.Context) error {
	ws, err := d.open(ctx)
	if err != nil {
		return err
	}
	return ws.Close()
}

// DialTarget opens a tunnel to host:port.
func (d *Dialer) DialTarget(ctx context.Context, host string, port int) (io.ReadWriteCloser, error) {
	req, err := encodeTarget(host, port)
	if err != nil {
		return nil, err
	}
	ws, err := d.open(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := ws.Write(req); err != nil {
		ws.Close()
		return nil, err
	}
	var st [1]byte
	_ = ws.c.SetReadDeadline(time.Now().Add(d.timeout() + 10*time.Second))
	if _, err := io.ReadFull(ws, st[:]); err != nil {
		ws.Close()
		return nil, fmt.Errorf("tunnel handshake: %w", err)
	}
	_ = ws.c.SetReadDeadline(time.Time{})
	if st[0] != 0 {
		ws.Close()
		return nil, errors.New("server could not reach the target")
	}
	return ws, nil
}
