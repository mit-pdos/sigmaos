package fslambda

import (
	"errors"
	"log"
	// "time"

	// db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfs"
	np "ulambda/ninep"
)

type Reader struct {
	*fslib.FsLibSrv
	pid    string
	input  string
	output string
	pipe   *memfs.Inode
}

func MakeReader(args []string) (*Reader, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeReader: too few arguments")
	}
	log.Printf("MakeReader: %v\n", args)

	fs := memfs.MakeRoot()
	n := "name/" + args[2]
	pipe, err := fs.MkPipe(n, fs.RootInode(), "pipe")
	if err != nil {
		log.Fatal("Create error: ", err)
	}

	fsl, err := fslib.InitFsMemFs(n, fs, nil)
	if err != nil {
		return nil, err
	}

	r := &Reader{}
	r.FsLibSrv = fsl
	r.pid = args[0]
	r.input = args[1]
	r.output = args[2]
	r.pipe = pipe

	r.Started(r.pid)

	return r, nil
}

func (r *Reader) Work() {
	err := r.pipe.Open(np.OWRITE)
	if err != nil {
		log.Fatal("Open error: ", err)
	}
	fd, err := r.Open(r.input, np.OREAD)
	if err != nil {
		log.Fatal(err)
	}
	for {
		data, err := r.Read(fd, memfs.PIPESZ)
		if len(data) == 0 {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		_, err = r.pipe.Write(0, data)
		if err != nil {
			log.Fatal(err)
		}
	}
	r.Close(fd)
	r.pipe.Close(np.OWRITE)

	r.ExitFs("name/" + r.output)
	r.Exiting(r.pid, "OK")
}
