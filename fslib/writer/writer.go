package writer

import (
	db "sigmaos/debug"
	sof "sigmaos/api/sigmaos"
	sp "sigmaos/sigmap"
)

type Writer struct {
	sof sof.FileAPI
	fd  int
	off sp.Toffset
}

func (wrt *Writer) Write(p []byte) (int, error) {
	n, err := wrt.sof.Write(wrt.fd, p)
	if err != nil {
		db.DPrintf(db.WRITER_ERR, "Write err %v" /*wrt.fc.Path(wrt.fid),*/, err)
		return 0, err
	}
	wrt.off += sp.Toffset(n)
	return int(n), nil
}

func (wrt *Writer) Close() error {
	err := wrt.sof.CloseFd(wrt.fd)
	if err != nil {
		return err
	}
	return nil
}

func (wrt *Writer) Nbytes() sp.Tlength {
	return sp.Tlength(wrt.off)
}

func NewWriter(sos sof.FileAPI, fd int) *Writer {
	return &Writer{sos, fd, 0}
}
