// Package fusionwire defines the bounded wire envelope shared by Go, C, and Kotlin.
// It is a framing primitive, not a complete authenticated tunnel or bonding protocol.
package fusionwire

import (
 "encoding/binary"
 "errors"
 "fmt"
)

const (
 Version = 1
 HeaderSize = 16
 MaxPayloadSize = 64 * 1024
)

var (
 ErrMalformedFrame = errors.New("malformed fusion frame")
 ErrPayloadTooLarge = errors.New("fusion payload exceeds limit")
)

type Type uint8
const (
 TypeData Type = 1
 TypePing Type = 2
 TypePong Type = 3
 TypeClose Type = 4
)

type Frame struct {
 Type Type
 Flags uint16
 Sequence uint32
 Payload []byte
}

func validType(t Type) bool { return t >= TypeData && t <= TypeClose }

// Encode serializes one frame using network byte order and returns an owned buffer.
func Encode(f Frame) ([]byte, error) {
 if !validType(f.Type) { return nil, fmt.Errorf("%w: unsupported type %d", ErrMalformedFrame, f.Type) }
 if len(f.Payload) > MaxPayloadSize { return nil, ErrPayloadTooLarge }
 out := make([]byte, HeaderSize+len(f.Payload))
 copy(out[:4], "PGF1")
 out[4], out[5] = Version, byte(f.Type)
 binary.BigEndian.PutUint16(out[6:8], f.Flags)
 binary.BigEndian.PutUint32(out[8:12], f.Sequence)
 binary.BigEndian.PutUint32(out[12:16], uint32(len(f.Payload)))
 copy(out[HeaderSize:], f.Payload)
 return out, nil
}

// Decode validates exactly one complete frame. Payload aliases wire; do not mutate wire while using it.
func Decode(wire []byte) (Frame, error) {
 if len(wire) < HeaderSize || string(wire[:4]) != "PGF1" || wire[4] != Version { return Frame{}, ErrMalformedFrame }
 t := Type(wire[5])
 if !validType(t) { return Frame{}, fmt.Errorf("%w: unsupported type %d", ErrMalformedFrame, t) }
 n := binary.BigEndian.Uint32(wire[12:16])
 if n > MaxPayloadSize { return Frame{}, ErrPayloadTooLarge }
 if uint64(len(wire)) != uint64(HeaderSize)+uint64(n) { return Frame{}, ErrMalformedFrame }
 return Frame{Type:t, Flags:binary.BigEndian.Uint16(wire[6:8]), Sequence:binary.BigEndian.Uint32(wire[8:12]), Payload:wire[HeaderSize:]}, nil
}
