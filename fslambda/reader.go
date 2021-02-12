package fslambda

import (
	"errors"
	"io"
	"log"
	"os"
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
	file, err := os.Open(r.input)
	if err != nil {
		log.Fatal(err)
	}
	for {
		data := make([]byte, memfs.PIPESZ)
		count, err := file.Read(data)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		_, err = r.pipe.Write(0, data[:count])
		if err != nil {
			log.Fatal(err)
		}
	}
	r.pipe.Close(np.OWRITE)

	r.ExitFs("name/" + r.output)
	r.Exiting(r.pid, "OK")
}
