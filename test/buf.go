package test

import (
	"fmt"
	"io"
	"testing"
)

func MkBuf(n int) []byte {
	buf := make([]byte, n)
	for i := 0; i < n; i++ {
		buf[i] = byte(i & 0xFF)
	}
	return buf
}

func Writer(t *testing.T, wrt io.Writer, buf []byte, fsz int) error {
	for n := 0; n < fsz; {
		w := len(buf)
		if fsz-n < w {
			w = fsz - n
		}
		m, err := wrt.Write(buf[0:w])
		if err != nil {
			return err
		}
		if w != m {
			return fmt.Errorf("short write %d %d", w, m)
		}
		n += m
	}
	return nil
}
