package reader

import (
	"io"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type ReaderI interface {
	Read(sp.Toffset, sp.Tsize) ([]byte, error)
	Close() error
}

type Reader struct {
	rdr    ReaderI
	path   string
	off    sp.Toffset
	eof    bool
	fenced bool
}

func (rdr *Reader) Path() string {
	return rdr.path
}

func (rdr *Reader) Nbytes() sp.Tlength {
	return sp.Tlength(rdr.off)
}

func (rdr *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if rdr.eof {
		return 0, io.EOF
	}
	var b []byte
	var err error
	sz := sp.Tsize(len(p))
	if rdr.fenced {
		b, err = rdr.rdr.Read(rdr.off, sz)
	} else {
		b, err = rdr.rdr.Read(rdr.off, sz)
	}
	if err != nil {
		db.DPrintf(db.READER_ERR, "Read %v err %v\n", rdr.path, err)
		return 0, err
	}
	if len(b) == 0 {
		rdr.eof = true
		return 0, io.EOF
	}
	if len(p) != len(b) {
		db.DPrintf(db.READER_ERR, "Read short %v %v %v\n", rdr.path, len(p), len(b))
	}
	// XXX change rdr.Read to avoid copy
	copy(p, b)
	rdr.off += sp.Toffset(len(b))
	return len(b), nil
}

func (rdr *Reader) GetData() ([]byte, error) {
	b, err := rdr.GetDataErr()
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (rdr *Reader) GetDataErr() ([]byte, error) {
	b, err := rdr.rdr.Read(0, sp.MAXGETSET)
	return b, err
}

func (rdr *Reader) Close() error {
	err := rdr.rdr.Close()
	if err != nil {
		return err
	}
	return nil
}

func (rdr *Reader) Unfence() {
	rdr.fenced = false
}

func (rdr *Reader) Reader() ReaderI {
	return rdr.rdr
}

func NewReader(rdr ReaderI, path string) *Reader {
	return &Reader{rdr, path, 0, false, true}
}
