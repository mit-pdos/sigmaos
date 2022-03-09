package reader

import (
	"io"

	"ulambda/fidclnt"
	np "ulambda/ninep"
)

type Reader struct {
	fc      *fidclnt.FidClnt
	path    string
	fid     np.Tfid
	buf     []byte
	off     np.Toffset
	eof     bool
	chunksz np.Tsize
}

func (rdr *Reader) Path() string {
	return rdr.path
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
	b, err := rdr.GetDataErr()
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (rdr *Reader) GetDataErr() ([]byte, *np.Err) {
	return rdr.fc.Read(rdr.fid, 0, np.MAXGETSET)
}

func (rdr *Reader) Close() error {
	err := rdr.fc.Clunk(rdr.fid)
	if err != nil {
		return err
	}
	return nil
}

func MakeReader(fc *fidclnt.FidClnt, path string, fid np.Tfid, chunksz np.Tsize) *Reader {
	return &Reader{fc, path, fid, make([]byte, 0), 0, false, chunksz}
}
