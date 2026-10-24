package native

import (
	"errors"
	"io"
	"net"
	"net/netip"
	"strconv"
)

// Tunnel request, first message on a new WebSocket:
//
//	atyp(1)  1=IPv4, 3=domain, 4=IPv6
//	addr     4 bytes | len(1)+name | 16 bytes
//	port(2)  big endian
//
// The server answers with one byte: 0 = connected, anything else = failed.
// After that both directions carry the raw byte stream.
const (
	atypIPv4   = 1
	atypDomain = 3
	atypIPv6   = 4
)

func encodeTarget(host string, port int) ([]byte, error) {
	if port < 1 || port > 65535 {
		return nil, errors.New("invalid port")
	}
	var b []byte
	if addr, err := netip.ParseAddr(host); err == nil {
		addr = addr.Unmap()
		if addr.Is4() {
			a := addr.As4()
			b = append([]byte{atypIPv4}, a[:]...)
		} else {
			a := addr.As16()
			b = append([]byte{atypIPv6}, a[:]...)
		}
	} else {
		if host == "" || len(host) > 255 {
			return nil, errors.New("invalid host")
		}
		b = append([]byte{atypDomain, byte(len(host))}, host...)
	}
	return append(b, byte(port>>8), byte(port)), nil
}

func readTarget(r io.Reader) (host string, port int, err error) {
	var t [1]byte
	if _, err = io.ReadFull(r, t[:]); err != nil {
		return
	}
	switch t[0] {
	case atypIPv4:
		var a [4]byte
		if _, err = io.ReadFull(r, a[:]); err == nil {
			host = netip.AddrFrom4(a).String()
		}
	case atypIPv6:
		var a [16]byte
		if _, err = io.ReadFull(r, a[:]); err == nil {
			host = netip.AddrFrom16(a).String()
		}
	case atypDomain:
		var l [1]byte
		if _, err = io.ReadFull(r, l[:]); err != nil {
			return
		}
		if l[0] == 0 {
			err = errors.New("empty host")
			return
		}
		name := make([]byte, l[0])
		if _, err = io.ReadFull(r, name); err == nil {
			host = string(name)
		}
	default:
		err = errors.New("unknown address type")
	}
	if err != nil {
		return
	}
	var p [2]byte
	if _, err = io.ReadFull(r, p[:]); err != nil {
		return
	}
	port = int(p[0])<<8 | int(p[1])
	if port == 0 {
		err = errors.New("invalid port")
	}
	return
}

func hostPort(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}
