package fslib

import (
	"bufio"
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

func (fl *FsLib) AppendFileJson(fname string, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	_, err = fl.SetFile(fname, data, np.OAPPEND, np.NoOffset)
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

func JsonReader(rdr io.Reader, mk func() interface{}, f func(i interface{}) error) error {
	return JsonBufReader(bufio.NewReader(rdr), mk, f)
}

func JsonBufReader(rdr *bufio.Reader, mk func() interface{}, f func(i interface{}) error) error {
	for {
		l, err := binary.ReadVarint(rdr)
		if err != nil && err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		data := make([]byte, l)
		if n, err := io.ReadFull(rdr, data); err != nil {
			return err
		} else if int64(n) != l {
			return fmt.Errorf("JsonBufReader: short read %v %v\n", n, l)
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
