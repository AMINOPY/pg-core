package fusionwire

import (
	"bytes"
	"errors"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	in := Frame{Type: TypeData, Flags: 3, Sequence: 0x10203040, Payload: []byte("hello")}
	wire, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(wire)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != in.Type || got.Flags != in.Flags || got.Sequence != in.Sequence || !bytes.Equal(got.Payload, in.Payload) {
		t.Fatalf("round trip mismatch: %#v", got)
	}
}

func TestDecodeRejectsInvalidFrames(t *testing.T) {
	good, err := Encode(Frame{Type: TypePing, Payload: []byte{1}})
	if err != nil {
		t.Fatal(err)
	}
	cases := [][]byte{nil, []byte("bad header bytes"), append([]byte(nil), good[:HeaderSize-1]...)}
	bad := append([]byte(nil), good...)
	bad[0] = 'X'
	cases = append(cases, bad)
	bad = append([]byte(nil), good...)
	bad[4]++
	cases = append(cases, bad)
	bad = append([]byte(nil), good...)
	bad[5] = 99
	cases = append(cases, bad)
	bad = append([]byte(nil), good...)
	bad[15]++
	cases = append(cases, bad)
	cases = append(cases, append(append([]byte(nil), good...), 0))
	for i, wire := range cases {
		if _, err := Decode(wire); !errors.Is(err, ErrMalformedFrame) {
			t.Errorf("case %d: err=%v, want ErrMalformedFrame", i, err)
		}
	}
}

func TestPayloadLimit(t *testing.T) {
	if _, err := Encode(Frame{Type: TypeData, Payload: make([]byte, MaxPayloadSize+1)}); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("Encode error=%v", err)
	}
	wire, err := Encode(Frame{Type: TypeData})
	if err != nil {
		t.Fatal(err)
	}
	wire[12], wire[13], wire[14], wire[15] = 0, 1, 0, 1
	if _, err := Decode(wire); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("Decode error=%v", err)
	}
}

func TestEncodeRejectsUnknownType(t *testing.T) {
	if _, err := Encode(Frame{Type: 0}); !errors.Is(err, ErrMalformedFrame) {
		t.Fatalf("error=%v", err)
	}
}

func TestSharedGoldenVector(t *testing.T) {
	want := []byte{
		0x50, 0x47, 0x46, 0x31, 0x01, 0x01, 0x00, 0x07,
		0x10, 0x20, 0x30, 0x40, 0x00, 0x00, 0x00, 0x02,
		0x68, 0x69,
	}
	wire, err := Encode(Frame{Type: TypeData, Flags: 7, Sequence: 0x10203040, Payload: []byte("hi")})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire, want) {
		t.Fatalf("wire = %x, want %x", wire, want)
	}
	got, err := Decode(want)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != TypeData || got.Flags != 7 || got.Sequence != 0x10203040 || !bytes.Equal(got.Payload, []byte("hi")) {
		t.Fatalf("decoded frame mismatch: %#v", got)
	}
}
