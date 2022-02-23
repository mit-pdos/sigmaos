package reader

import (
	"io"

	"ulambda/fidclnt"
	np "ulambda/ninep"
)

type Reader struct {
	fc      *fidclnt.FidClient
	fid     np.Tfid
	buf     []byte
	off     np.Toffset
	eof     bool
	chunksz np.Tsize
}

func (rdr *Reader) ReadByte() (byte, error) {
	d := make([]byte, 1)
	_, err := rdr.Read(d)
	if err != nil {
		return 0, err
	}
	return d[0], nil
}

func (rdr *Reader) Read(p []byte) (int, error) {
	for len(p) > len(rdr.buf) && !rdr.eof {
		b, err := rdr.fc.Read(rdr.fid, rdr.off, rdr.chunksz)
		if err != nil {
			return -1, err
		}
		if len(b) == 0 {
			rdr.eof = true
		}
		rdr.off += np.Toffset(len(b))
		rdr.buf = append(rdr.buf, b...)
	}
	if len(rdr.buf) == 0 {
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
	return rdr.fc.Read(rdr.fid, 0, np.MAXGETSET)
}

func (rdr *Reader) Close() error {
	return rdr.fc.Close(rdr.fid)
}

func MakeReader(fc *fidclnt.FidClient, fid np.Tfid, chunksz np.Tsize) (*Reader, error) {
	return &Reader{fc, fid, make([]byte, 0), 0, false, chunksz}, nil
}
