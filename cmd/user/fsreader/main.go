package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

//
// Creates a named pipe in name/<name>/pipe (name is os.Args[1]), reads a
// data from input (os.Args[2])), and writes it to the named pipe.
//

func main() {
	r, err := MakeReader(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	s := r.Work()
	r.Exit(s)
}

type Reader struct {
	*fslib.FsLib
	proc.ProcClnt
	input string
	pipe  fs.FsObj
}

func MakeReader(args []string) (*Reader, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeReader: too few arguments")
	}
	log.Printf("MakeReader: %v\n", args)
	r := &Reader{}
	r.FsLib = fslib.MakeFsLib("fsreader")
	r.ProcClnt = procclnt.MakeProcClnt(r.FsLib)
	n := "pids/" + args[1] + "/server"
	mfs, err := fslibsrv.StartMemFsFsl(n, r.FsLib)
	if err != nil {
		log.Fatalf("MakeSrvFsLib %v\n", err)
	}
	r.pipe, err = mfs.Root().Create(fssrv.MkCtx(""), "pipe", np.DMNAMEDPIPE, 0)
	if err != nil {
		log.Fatal("Create error: ", err)
	}
	r.input = args[2]
	r.Started(proc.GetPid())
	return r, nil
}

// Open r.pipe before opening r.input so that when open r.input fails
// we will close r.pipe causing the reader to stop waiting on the
// pipe, and can pick up the exit status "File not found".
func (r *Reader) Work() string {
	db.DLPrintf("Reader", "Reader: work\n")
	_, err := r.pipe.Open(nil, np.OWRITE)
	if err != nil {
		log.Fatal("Open error: ", err)
	}
	defer r.pipe.Close(nil, np.OWRITE)
	fd, err := r.Open(r.input, np.OREAD)
	if err != nil {
		return "File not found"
	}
	for {
		data, err := r.Read(fd, memfs.PIPESZ)
		if len(data) == 0 || err != nil {
			break
		}
		_, err = r.pipe.(fs.File).Write(nil, 0, data, np.NoV)
		if err != nil {
			log.Fatal(err)
		}
	}
	r.Close(fd)
	return "OK"
}

func (r *Reader) Exit(status string) {
	r.ShutdownFs("name/" + proc.GetPid())
	r.Exited(proc.GetPid(), status)
}
