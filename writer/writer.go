package writer

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

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
	n, err := wrt.fc.WriteV(wrt.fid, wrt.off, p, np.NoV)
	if err != nil {
		return 0, err
	}
	wrt.off += np.Toffset(n)
	return int(n), nil
}

func JsonRecord(a interface{}) ([]byte, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	lbuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(lbuf, int64(len(b)))
	b = append(lbuf[0:n], b...)
	return b, nil
}

func (wrt *Writer) WriteJsonRecord(r interface{}) error {
	b, err := JsonRecord(r)
	if err != nil {
		return err
	}
	_, err = wrt.Write(b)
	if err != nil {
		return fmt.Errorf("WriteJsonRecord write err %v", err)
	}
	return nil
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
