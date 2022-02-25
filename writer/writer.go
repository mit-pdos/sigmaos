package writer

import (
	"ulambda/fidclnt"
	np "ulambda/ninep"
)

type Writer struct {
	fc      *fidclnt.FidClnt
	fid     np.Tfid
	buf     []byte
	off     np.Toffset
	eof     bool
	chunksz np.Tsize
}

func (wrt *Writer) Write(p []byte) (int, error) {
	n, err := wrt.fc.Write(wrt.fid, wrt.off, p)
	if err != nil {
		return 0, nil
	}
	wrt.off += np.Toffset(n)
	return int(n), nil
}

func (wrt *Writer) Close() error {
	err := wrt.fc.Clunk(wrt.fid)
	if err != nil {
		return err
	}
	return nil
}

func MakeWriter(fc *fidclnt.FidClnt, fid np.Tfid, chunksz np.Tsize) *Writer {
	return &Writer{fc, fid, make([]byte, 0), 0, false, chunksz}
}
