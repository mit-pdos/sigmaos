package writer

import (
	db "sigmaos/debug"
	"sigmaos/fidclnt"
	sp "sigmaos/sigmap"
)

type Writer struct {
	fc  *fidclnt.FidClnt
	fid sp.Tfid
	off sp.Toffset
}

func (wrt *Writer) Write(p []byte) (int, error) {
	n, err := wrt.fc.WriteV(wrt.fid, wrt.off, p, sp.NoV)
	if err != nil {
		db.DPrintf(db.WRITER_ERR, "Write %v err %v\n", wrt.fc.Path(wrt.fid), err)
		return 0, err
	}
	wrt.off += sp.Toffset(n)
	return int(n), nil
}

func (wrt *Writer) Close() error {
	err := wrt.fc.Clunk(wrt.fid)
	if err != nil {
		return err
	}
	return nil
}

func (wrt *Writer) Nbytes() sp.Tlength {
	return sp.Tlength(wrt.off)
}
func MakeWriter(fc *fidclnt.FidClnt, fid sp.Tfid) *Writer {
	return &Writer{fc, fid, 0}
}
