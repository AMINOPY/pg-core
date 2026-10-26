package native

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const udpModeMarker byte = 0xf0

// DialDatagram sends one UDP datagram through the native WebSocket tunnel and
// returns the first response datagram. This protocol is supported by the Go
// Server implementation; Cloudflare Workers' TCP-only socket API does not
// currently implement this UDP mode.
func (d *Dialer) DialDatagram(ctx context.Context, host string, port int, payload []byte) ([]byte, error) {
	if len(payload) == 0 || len(payload) > 65507 {
		return nil, errors.New("UDP payload must be between 1 and 65507 bytes")
	}
	target, err := encodeTarget(host, port)
	if err != nil {
		return nil, err
	}
	ws, err := d.open(ctx)
	if err != nil {
		return nil, err
	}
	defer ws.Close()

	request := make([]byte, 1+len(target))
	request[0] = udpModeMarker
	copy(request[1:], target)
	if err := writeAll(ws, request); err != nil {
		return nil, err
	}
	var status [1]byte
	if _, err := io.ReadFull(ws, status[:]); err != nil {
		return nil, fmt.Errorf("UDP tunnel handshake: %w", err)
	}
	if status[0] != 0 {
		return nil, errors.New("server could not open UDP target")
	}
	var size [2]byte
	binary.BigEndian.PutUint16(size[:], uint16(len(payload)))
	if err := writeAll(ws, size[:]); err != nil {
		return nil, err
	}
	if err := writeAll(ws, payload); err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = ws.c.SetReadDeadline(deadline)
	}
	stopCancel := context.AfterFunc(ctx, func() { _ = ws.c.SetReadDeadline(time.Now()) })
	defer stopCancel()
	if _, err := io.ReadFull(ws, size[:]); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("UDP response length: %w", err)
	}
	n := int(binary.BigEndian.Uint16(size[:]))
	if n == 0 {
		return nil, errors.New("empty UDP response")
	}
	response := make([]byte, n)
	if _, err := io.ReadFull(ws, response); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("UDP response: %w", err)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return response, nil
}
