package fslib

import (
	"encoding/json"
	"fmt"

	"ulambda/fsclnt"
	np "ulambda/ninep"
	"ulambda/reader"
)

func (fl *FsLib) ReadSeqNo() np.Tseqno {
	return fl.FsClient.ReadSeqNo()
}

func (fl *FsLib) MakeReader(path string) (*reader.Reader, error) {
	return reader.MakeReaderWatch(fl.FsClient, path, nil, fl.chunkSz)
}

func (fl *FsLib) MakeReaderWatch(path string, f fsclnt.Watch) (*reader.Reader, error) {
	return reader.MakeReaderWatch(fl.FsClient, path, f, fl.chunkSz)
}

func (fl *FsLib) GetFileWatch(path string, f fsclnt.Watch) ([]byte, error) {
	rdr, err := reader.MakeReaderWatch(fl.FsClient, path, f, fl.chunkSz)
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	b, err := rdr.GetData()
	return b, err
}

func (fl *FsLib) GetFileJsonWatch(name string, i interface{}, f fsclnt.Watch) error {
	b, err := fl.GetFileWatch(name, f)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, i)
}

func (fl *FsLib) GetFile(fname string) ([]byte, error) {
	return fl.FsClient.GetFile(fname, np.OREAD, 0, np.MAXGETSET)
}

func (fl *FsLib) SetFile(fname string, data []byte, off np.Toffset) (np.Tsize, error) {
	return fl.FsClient.SetFile(fname, np.OWRITE, data, off)
}

func (fl *FsLib) PutFile(fname string, perm np.Tperm, mode np.Tmode, data []byte) (np.Tsize, error) {
	return fl.FsClient.PutFile(fname, mode|np.OWRITE, perm, data, 0)
}

func (fl *FsLib) CreateFile(fname string, perm np.Tperm, mode np.Tmode) (int, error) {
	fd, err := fl.Create(fname, perm, mode)
	if err != nil {
		return -1, err
	}
	return fd, nil
}

func (fl *FsLib) CopyFile(src, dst string) error {
	st, err := fl.Stat(src)
	if err != nil {
		return err
	}
	fdsrc, err := fl.Open(src, np.OREAD)
	if err != nil {
		return err
	}
	defer fl.Close(fdsrc)
	fddst, err := fl.Create(dst, st.Mode, np.OWRITE)
	if err != nil {
		return err
	}
	defer fl.Close(fddst)
	for {
		b, err := fl.Read(fdsrc, fl.chunkSz)
		if err != nil {
			return err
		}
		if len(b) == 0 {
			break
		}
		n, err := fl.Write(fddst, b)
		if err != nil {
			return err
		}
		if n != np.Tsize(len(b)) {
			return fmt.Errorf("short write")
		}
	}
	return nil
}

func (fl *FsLib) GetFileJson(name string, i interface{}) error {
	b, err := fl.GetFile(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, i)
}

func (fl *FsLib) PutFileJson(fname string, perm np.Tperm, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	_, err = fl.PutFile(fname, perm, np.OWRITE, data)
	return err
}

func (fl *FsLib) SetFileJson(fname string, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	_, err = fl.SetFile(fname, data, 0)
	return err
}
