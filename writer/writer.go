package writer

import (
	db "sigmaos/debug"
	"sigmaos/fidclnt"
	np "sigmaos/ninep"
)

type Writer struct {
	fc  *fidclnt.FidClnt
	fid np.Tfid
	off np.Toffset
}

func (wrt *Writer) Write(p []byte) (int, error) {
	n, err := wrt.fc.WriteV(wrt.fid, wrt.off, p, np.NoV)
	if err != nil {
		db.DPrintf("WRITER_ERR", "Write %v err %v\n", wrt.fc.Path(wrt.fid), err)
		return 0, err
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

func (wrt *Writer) Nbytes() np.Tlength {
	return np.Tlength(wrt.off)
}
func MakeWriter(fc *fidclnt.FidClnt, fid np.Tfid) *Writer {
	return &Writer{fc, fid, 0}
}
