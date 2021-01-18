package fslambda

import (
	"errors"
	// "fmt"
	"io"
	"log"
	"os"

	"ulambda/fslib"
	"ulambda/memfs"
	"ulambda/memfsd"
	// np "ulambda/ninep"
	"ulambda/npsrv"
)

type Reader struct {
	clnt   *fslib.FsLib
	srv    *npsrv.NpServer
	memfsd *memfsd.Fsd
	pid    string
	input  string
	output string
	pipe   *memfs.Inode
}

func MakeReader(args []string) (*Reader, error) {
	r := &Reader{}
	r.clnt = fslib.MakeFsLib(false)
	r.memfsd = memfsd.MakeFsd(false)
	r.srv = npsrv.MakeNpServer(r.memfsd, ":0", false)
	if len(args) != 3 {
		return nil, errors.New("MakeReader: too few arguments")
	}
	log.Printf("MakeReader: %v\n", args)
	r.pid = args[0]
	r.input = args[1]
	r.output = args[2]

	if fd, err := r.clnt.Attach(":1111", ""); err == nil {
		err := r.clnt.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
		err = r.clnt.Remove("name/" + r.output)
		if err != nil {
			log.Printf("Remove failed %v\n", err)
		}

		// XXX use local interface for MkPipe
		fs := r.memfsd.Root()
		r.pipe, err = fs.MkPipe(fs.RootInode(), "pipe")
		if err != nil {
			log.Fatal("Create error: ", err)
		}

		name := r.srv.MyAddr()
		err = r.clnt.Symlink(name+":pubkey:pipe", "name/"+r.output, 0777)
		if err != nil {
			log.Fatal("Symlink error: ", err)
		}
		r.clnt.Started(r.pid)
	} else {
		log.Fatal("Attach error: ", err)
	}

	return r, nil
}

func (r *Reader) Work() {
	file, err := os.Open(r.input)
	if err != nil {
		log.Fatal(err)
	}
	for {
		data := make([]byte, 8192)
		count, err := file.Read(data)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		// fmt.Printf("read %d bytes\n", count)
		_, err = r.pipe.Write(0, data[:count])
		if err != nil {
			log.Fatal(err)
		}
	}
	r.clnt.Exit(r.pid)
}
