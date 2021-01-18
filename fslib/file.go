package fslib

import (
	"log"

	"ulambda/fsclnt"
	np "ulambda/ninep"
)

type FsLib struct {
	*fsclnt.FsClient
}

func MakeFsLib(d bool) *FsLib {
	fl := &FsLib{fsclnt.MakeFsClient(d)}
	if fd, err := fl.Attach(":1111", ""); err == nil {
		err := fl.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
	}
	return fl
}

func (fl *FsLib) ReadFile(fname string) ([]byte, error) {
	const CNT = 8192
	fd, err := fl.Open(fname, np.OREAD)
	if err != nil {
		log.Printf("open failed %v %v\n", fname, err)
		return nil, err
	}
	c := []byte{}
	for {
		b, err := fl.Read(fd, CNT)
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

func (fl *FsLib) WriteFile(fname string, data []byte) error {
	fd, err := fl.Open(fname, np.OWRITE)
	if err != nil {
		log.Printf("open failed %v %v\n", fname, err)
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

func (fl *FsLib) MakeFile(fname string, data []byte) error {
	log.Printf("makeFile %v\n", fname)
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
	log.Printf("CreateFile %v %v\n", fname, err)
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
