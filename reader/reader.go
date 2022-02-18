package reader

import (
	"io"

	"ulambda/fslib"
	np "ulambda/ninep"
)

const CHUNKSZ = 4096

type Reader struct {
	fl  *fslib.FsLib
	fd  int
	buf []byte
	eof bool
}

func (rd *Reader) ReadByte() (byte, error) {
	d := make([]byte, 1)
	_, err := rd.Read(d)
	if err != nil {
		return 0, err
	}
	return d[0], nil
}

func (rd *Reader) Read(p []byte) (int, error) {
	for len(p) > len(rd.buf) && !rd.eof {
		b, err := rd.fl.Read(rd.fd, CHUNKSZ)
		if err != nil {
			return -1, err
		}
		if len(b) == 0 {
			rd.eof = true
		}
		rd.buf = append(rd.buf, b...)
	}
	if len(rd.buf) == 0 {
		rd.fl.Close(rd.fd)
		return 0, io.EOF
	}
	max := len(p)
	if len(rd.buf) < max {
		max = len(rd.buf)
	}
	copy(p, rd.buf)
	rd.buf = rd.buf[max:]
	return max, nil
}

func (rd *Reader) Close() error {
	return rd.fl.Close(rd.fd)
}

func MakeReader(fl *fslib.FsLib, path string) (*Reader, error) {
	fd, err := fl.Open(path, np.OREAD)
	if err != nil {
		return nil, err
	}
	return &Reader{fl, fd, make([]byte, 0), false}, nil
}
