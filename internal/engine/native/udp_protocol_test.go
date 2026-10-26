package native

import (
	"context"
	"io"
	"testing"
	"time"
)

func TestNativeUDPRejectsUnsupportedProtocolVersion(t *testing.T) {
	e := newEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ws, err := e.dialer("127.0.0.1").open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	if err := ws.c.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	// The first byte selects UDP mode; the next byte is the protocol version.
	if err := writeAll(ws, []byte{udpModeMarker, udpProtocolVersion + 1}); err != nil {
		t.Fatal(err)
	}
	var status [1]byte
	if _, err := io.ReadFull(ws, status[:]); err != nil {
		t.Fatal(err)
	}
	if status[0] != 2 {
		t.Fatalf("unsupported-version status = %d, want 2", status[0])
	}
}
