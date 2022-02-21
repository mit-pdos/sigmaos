package writer

import (
	"ulambda/fsclnt"
	np "ulambda/ninep"
)

type Writer struct {
	fc      *fsclnt.FsClient
	fd      int
	buf     []byte
	eof     bool
	chunksz np.Tsize
}

func (wrt *Writer) Write(p []byte) (int, error) {
	n, err := wrt.fc.Write(wrt.fd, p)
	return int(n), err
}

func (wrt *Writer) Close() error {
	return wrt.fc.Close(wrt.fd)
}

func MakeWriter(fc *fsclnt.FsClient, fd int, chunksz np.Tsize) (*Writer, error) {
	return &Writer{fc, fd, make([]byte, 0), false, chunksz}, nil
}
