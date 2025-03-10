// Package reader wraps callers that have ReadOffsetI and returns an
// io.Reader interface
package reader

import (
	"io"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type ReadOffsetI interface {
	Read(sp.Toffset, []byte) (int, error)
	Close() error
}

type Reader struct {
	rdr ReadOffsetI
	off sp.Toffset
	eof bool
}

func (rdr *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if rdr.eof {
		return 0, io.EOF
	}
	n, err := rdr.rdr.Read(rdr.off, p)
	if err != nil {
		db.DPrintf(db.READER_ERR, "Read err %v\n", err)
		return 0, err
	}
	if n == 0 {
		rdr.eof = true
		return 0, io.EOF
	}
	if int(n) < len(p) {
		db.DPrintf(db.READER_ERR, "Read short %v %v\n", len(p), n)
	}
	rdr.off += sp.Toffset(n)
	return int(n), nil
}

func (rdr *Reader) Close() error {
	err := rdr.rdr.Close()
	if err != nil {
		return err
	}
	return nil
}

func NewReader(rdr ReadOffsetI, pn sp.Tsigmapath) *Reader {
	return &Reader{rdr, 0, false}
}
