package fslib

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	np "ulambda/ninep"
	"ulambda/reader"
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
	_, err = fl.SetFile(fname, data, 0)
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

func (fl *FsLib) ReadJsonStream(rdr *reader.Reader, mk func() interface{}, f func(i interface{}) error) error {
	for {
		l, err := binary.ReadVarint(rdr)
		if err != nil && err == io.EOF {
			break
		}
		data := make([]byte, l)
		if _, err := rdr.Read(data); err != nil {
			return err
		}
		v := mk()
		if err := json.Unmarshal(data, v); err != nil {
			return err
		}
		if err := f(v); err != nil {
			return err
		}
	}
	return nil
}
