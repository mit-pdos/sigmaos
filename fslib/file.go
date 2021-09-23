package fslib

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"ulambda/fsclnt"
	np "ulambda/ninep"
)

// XXX Picking a small chunk size really kills throughput
//const CHUNKSZ = 8192
const CHUNKSZ = 10000000

type FsLib struct {
	*fsclnt.FsClient
}

func Named() []string {
	named := os.Getenv("NAMED")
	if named == "" {
		log.Fatal("Getenv error: missing NAMED")
	}
	nameds := strings.Split(named, ",")
	return nameds
}

func MakeFsLibAddr(uname string, namedAddr []string) *FsLib {
	fl := &FsLib{fsclnt.MakeFsClient(uname)}
	if fd, err := fl.AttachReplicas(namedAddr, ""); err == nil {
		err := fl.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
	}
	return fl
}

func MakeFsLib(uname string) *FsLib {
	fl := &FsLib{fsclnt.MakeFsClient(uname)}
	if fd, err := fl.AttachReplicas(Named(), ""); err == nil {
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

func (fl *FsLib) GetFile(fname string) ([]byte, np.TQversion, error) {
	return fl.FsClient.GetFile(fname, np.OREAD)
}

func (fl *FsLib) SetFile(fname string, data []byte, version np.TQversion) (np.Tsize, error) {
	return fl.FsClient.SetFile(fname, np.OWRITE, 0, version, data)
}

func (fl *FsLib) PutFile(fname string, data []byte, perm np.Tperm, mode np.Tmode) (np.Tsize, error) {
	return fl.FsClient.SetFile(fname, mode|np.OWRITE, perm, np.NoV, data)
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
func (fl *FsLib) MakeFile(fname string, perm np.Tperm, mode np.Tmode, data []byte) error {
	_, err := fl.PutFile(fname, data, perm, mode)
	return err
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
	fdsrc, err := fl.OpenWatch(src, np.OREAD, nil)
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
		b, err := fl.Read(fdsrc, CHUNKSZ)
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
	return fl.MakeFile(fname, perm, np.OWRITE, data)
}

func (fl *FsLib) WriteFileJson(fname string, i interface{}) error {
	data, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	return fl.WriteFile(fname, data)
}
