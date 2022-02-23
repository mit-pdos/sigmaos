package writer

import (
	"ulambda/fidclnt"
	np "ulambda/ninep"
)

type Writer struct {
	fc      *fidclnt.FidClient
	fid     np.Tfid
	buf     []byte
	off     np.Toffset
	eof     bool
	chunksz np.Tsize
}

func (wrt *Writer) Write(p []byte) (int, error) {
	n, err := wrt.fc.Write(wrt.fid, wrt.off, p)
	wrt.off += np.Toffset(n)
	return int(n), err
}

func (wrt *Writer) Close() error {
	return wrt.fc.Close(wrt.fid)
}

func MakeWriter(fc *fidclnt.FidClient, fid np.Tfid, chunksz np.Tsize) (*Writer, error) {
	return &Writer{fc, fid, make([]byte, 0), 0, false, chunksz}, nil
}
