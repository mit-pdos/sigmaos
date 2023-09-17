package test

import (
	"fmt"
	"io"
	"testing"

	sp "sigmaos/sigmap"
)

func NewBuf(n int) []byte {
	buf := make([]byte, n)
	for i := 0; i < n; i++ {
		buf[i] = byte(i & 0xFF)
	}
	return buf
}

func Writer(t *testing.T, wrt io.Writer, buf []byte, fsz sp.Tlength) error {
	for n := sp.Tlength(0); n < fsz; {
		w := sp.Tlength(len(buf))
		if fsz-n < w {
			w = fsz - n
		}
		m, err := wrt.Write(buf[0:w])
		if err != nil {
			return err
		}
		if w != sp.Tlength(m) {
			return fmt.Errorf("short write %d %d", w, m)
		}
		n += sp.Tlength(m)
	}
	return nil
}

func Reader(t *testing.T, rdr io.Reader, buf []byte, sz sp.Tlength) (sp.Tlength, error) {
	s := 0
	for {
		m, err := rdr.Read(buf)
		s += m
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}
	if sp.Tlength(s) != sz {
		return 0, fmt.Errorf("short read %d %d", s, sz)
	}
	return sz, nil
}
