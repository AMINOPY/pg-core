package native

import "io"

// pipe copies both directions until both finish; an error closes both sides.
func pipe(a, b io.ReadWriteCloser) {
	done := make(chan struct{}, 2)
	cp := func(dst, src io.ReadWriteCloser) {
		_, err := io.Copy(dst, src)
		if cw, ok := dst.(interface{ CloseWrite() error }); ok && err == nil {
			_ = cw.CloseWrite()
		} else {
			_ = dst.Close()
			_ = src.Close()
		}
		done <- struct{}{}
	}
	go cp(a, b)
	go cp(b, a)
	<-done
	<-done
	_ = a.Close()
	_ = b.Close()
}
