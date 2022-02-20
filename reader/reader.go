package reader

import (
	"io"

	"ulambda/fsclnt"
	np "ulambda/ninep"
)

type Reader struct {
	fc      *fsclnt.FsClient
	fd      int
	buf     []byte
	eof     bool
	chunksz np.Tsize
}

func (rdr *Reader) ReadByte() (byte, error) {
	d := make([]byte, 1)
	_, err := rdr.Read(d)
	if err != nil {
		rdr.fc.Close(rdr.fd)
		return 0, err
	}
	return d[0], nil
}

func (rdr *Reader) Read(p []byte) (int, error) {
	for len(p) > len(rdr.buf) && !rdr.eof {
		b, err := rdr.fc.Read(rdr.fd, rdr.chunksz)
		if err != nil {
			rdr.fc.Close(rdr.fd)
			return -1, err
		}
		if len(b) == 0 {
			rdr.eof = true
		}
		rdr.buf = append(rdr.buf, b...)
	}
	if len(rdr.buf) == 0 {
		rdr.fc.Close(rdr.fd)
		return 0, io.EOF
	}
	max := len(p)
	if len(rdr.buf) < max {
		max = len(rdr.buf)
	}
	// XXX maybe don't copy: p = rdr.buf[0:max]
	copy(p, rdr.buf)
	rdr.buf = rdr.buf[max:]
	return max, nil
}

func (rdr *Reader) GetData() ([]byte, error) {
	return rdr.fc.Read(rdr.fd, np.MAXGETSET)
}

func (rdr *Reader) Close() error {
	return rdr.fc.Close(rdr.fd)
}

func MakeReader(fc *fsclnt.FsClient, fd int, chunksz np.Tsize) (*Reader, error) {
	return &Reader{fc, fd, make([]byte, 0), false, chunksz}, nil
}
