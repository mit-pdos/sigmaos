package fslib

import (
	"encoding/json"
	"fmt"
	"log"

	"ulambda/fsclnt"
	np "ulambda/ninep"
)

const CHUNKSZ = 8192

type FsLib struct {
	*fsclnt.FsClient
}

func MakeFsLib(uname string) *FsLib {
	fl := &FsLib{fsclnt.MakeFsClient(uname)}
	if fd, err := fl.Attach(":1111", ""); err == nil {
		err := fl.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
	}
	return fl
}

func (fl *FsLib) ReadFile(fname string) ([]byte, error) {
	fd, err := fl.Open(fname, np.OREAD)
	if err != nil {
		return nil, err
	}
	c := []byte{}
	for {
		b, err := fl.Read(fd, CHUNKSZ)
		if err != nil {
			return nil, err
		}
		if len(b) == 0 {
			break
		}
		c = append(c, b...)
	}
	err = fl.Close(fd)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// XXX chunk
func (fl *FsLib) WriteFile(fname string, data []byte) error {
	fd, err := fl.Open(fname, np.OWRITE)
	if err != nil {
		return err
	}
	_, err = fl.Write(fd, data)
	if err != nil {
		return err
	}
	err = fl.Close(fd)
	if err != nil {
		return err
	}
	return nil
}

// XXX chunk
func (fl *FsLib) MakeFile(fname string, data []byte) error {
	fd, err := fl.Create(fname, 0700, np.OWRITE)
	if err != nil {
		return err
	}
	_, err = fl.Write(fd, data)
	if err != nil {
		return err
	}
	return fl.Close(fd)
}

func (fl *FsLib) CreateFile(fname string, mode np.Tmode) (int, error) {
	fd, err := fl.Create(fname, 0700, mode)
	if err != nil {
		return -1, err
	}
	return fd, nil
}

func (fl *FsLib) isEmpty(name string) bool {
	st, err := fl.Stat(name)
	if err != nil {
		log.Fatalf("Stat %v error %v\n", name, err)
	}
	return st.Length == 0
}

func (fl *FsLib) ReadFileJson(name string, i interface{}) error {
	b, err := fl.ReadFile(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, i)
}

func (fl *FsLib) MakeFileJson(fname string, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	return fl.MakeFile(fname, data)
}

func (fl *FsLib) WriteFileJson(fname string, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	return fl.WriteFile(fname, data)
}
