package fslib

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"ulambda/fsclnt"
	np "ulambda/ninep"
)

// XXX Picking a small chunk size really kills throughput
//const CHUNKSZ = 8192
const CHUNKSZ = 10000000

type FsLib struct {
	*fsclnt.FsClient
}

func Named() string {
	named := os.Getenv("NAMED")
	if named == "" {
		log.Fatal("Getenv error: missing NAMED")
	}
	return named
}

func MakeFsLib(uname string) *FsLib {
	fl := &FsLib{fsclnt.MakeFsClient(uname)}
	if fd, err := fl.Attach(Named(), ""); err == nil {
		err := fl.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
	}
	return fl
}

func (fl *FsLib) readFile(fname string, m np.Tmode, f fsclnt.Watch) ([]byte, error) {
	fd, err := fl.OpenWatch(fname, np.OREAD|m, f)
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

func (fl *FsLib) ReadFile(fname string) ([]byte, error) {
	return fl.readFile(fname, 0x0, nil)
}

func (fl *FsLib) ReadFileWatch(fname string, f fsclnt.Watch) ([]byte, error) {
	return fl.readFile(fname, 0x0, f)
}

func (fl *FsLib) Get(fname string) ([]byte, error) {
	return fl.readFile(fname, np.OVERSION, nil)
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
func (fl *FsLib) MakeFile(fname string, perm np.Tperm, data []byte) error {
	fd, err := fl.Create(fname, perm, np.OWRITE)
	if err != nil {
		return err
	}
	_, err = fl.Write(fd, data)
	if err != nil {
		return err
	}
	return fl.Close(fd)
}

func (fl *FsLib) CreateFile(fname string, perm np.Tperm, mode np.Tmode) (int, error) {
	fd, err := fl.Create(fname, perm, mode)
	if err != nil {
		return -1, err
	}
	return fd, nil
}

func (fl *FsLib) IsDir(name string) (bool, error) {
	st, err := fl.Stat(name)
	if err != nil {
		return false, err
	}
	return st.Mode.IsDir(), nil
}

func (fl *FsLib) ReadFileJson(name string, i interface{}) error {
	b, err := fl.ReadFile(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, i)
}

func (fl *FsLib) ReadFileJsonWatch(name string, i interface{}, f fsclnt.Watch) error {
	b, err := fl.ReadFileWatch(name, f)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, i)
}

func (fl *FsLib) MakeFileJson(fname string, perm np.Tperm, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	return fl.MakeFile(fname, perm, data)
}

func (fl *FsLib) WriteFileJson(fname string, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	return fl.WriteFile(fname, data)
}
