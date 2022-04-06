package fslib

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	np "ulambda/ninep"
)

func (fl *FsLib) GetFileJson(name string, i interface{}) error {
	b, err := fl.GetFile(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, i)
}

func (fl *FsLib) SetFileJson(fname string, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	_, err = fl.SetFile(fname, data, np.OWRITE, 0)
	return err
}

func (fl *FsLib) PutFileJson(fname string, perm np.Tperm, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	_, err = fl.PutFile(fname, perm, np.OWRITE, data)
	return err
}

func (fl *FsLib) GetFileJsonWatch(name string, i interface{}) error {
	b, err := fl.GetFileWatch(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, i)
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

func WriteJsonRecord(wrt io.Writer, r interface{}) error {
	b, err := JsonRecord(r)
	if err != nil {
		return err
	}
	n, err := wrt.Write(b)
	if err != nil {
		return fmt.Errorf("WriteJsonRecord write err %v", err)
	}
	if n != len(b) {
		return fmt.Errorf("WriteJsonRecord short write %v", n)
	}
	return nil
}
