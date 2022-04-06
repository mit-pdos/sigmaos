package writer

import (
	db "ulambda/debug"
	"ulambda/fidclnt"
	np "ulambda/ninep"
)

type Writer struct {
	fc  *fidclnt.FidClnt
	fid np.Tfid
	buf []byte
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

func MakeWriter(fc *fidclnt.FidClnt, fid np.Tfid) *Writer {
	return &Writer{fc, fid, make([]byte, 0), 0}
}
