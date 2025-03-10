package fslib

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	sp "sigmaos/sigmap"
)

func (fl *FsLib) GetFileJson(pn sp.Tsigmapath, i interface{}) error {
	b, err := fl.GetFile(pn)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, i)
}

func (fl *FsLib) SetFileJson(pn sp.Tsigmapath, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	_, err = fl.SetFile(pn, data, sp.OWRITE, 0)
	return err
}

func (fl *FsLib) AppendFileJson(pn sp.Tsigmapath, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	_, err = fl.SetFile(pn, data, sp.OAPPEND, sp.NoOffset)
	return err
}

func (fl *FsLib) PutFileJson(pn sp.Tsigmapath, perm sp.Tperm, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	_, err = fl.PutFile(pn, perm, sp.OWRITE, data)
	return err
}

func (fl *FsLib) GetFileJsonWatch(pn sp.Tsigmapath, i interface{}) error {
	b, err := fl.GetFileWatch(pn)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, i)
}

func WriteJsonRecord(wrt io.Writer, r interface{}) error {
	b, err := json.Marshal(r)
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

func JsonReader(rdr io.Reader, new func() interface{}, f func(i interface{}) error) error {
	return JsonBufReader(bufio.NewReader(rdr), new, f)
}

func JsonBufReader(rdr *bufio.Reader, new func() interface{}, f func(i interface{}) error) error {
	dec := json.NewDecoder(rdr)
	return RecordReader(dec.Decode, new, f)
}

func RecordReader(decodefn func(interface{}) error, new func() interface{}, f func(i interface{}) error) error {
	for {
		v := new()
		if err := decodefn(&v); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if err := f(v); err != nil {
			return err
		}
	}
	return nil
}
