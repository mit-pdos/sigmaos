package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

//
// Reads data from input (os.Args[2])), and writes it to the named pipe found
// at proc.PARENTDIR/proc.SHARED
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
	*procclnt.ProcClnt
	input  string
	output string
	ctx    fs.CtxI
}

func MakeReader(args []string) (*Reader, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeReader: too few arguments")
	}
	log.Printf("MakeReader %v: %v\n", proc.GetPid(), args)
	r := &Reader{}
	db.Name("fsreader-" + proc.GetPid())
	r.FsLib = fslib.MakeFsLib("fsreader")
	r.ProcClnt = procclnt.MakeProcClnt(r.FsLib)
	r.input = args[2]
	r.output = path.Join(proc.PARENTDIR, proc.SHARED) + "/"
	r.Started(proc.GetPid())
	return r, nil
}

func (r *Reader) Work() string {
	db.DLPrintf("Reader", "Reader: work\n")
	// Open the pipe.
	pipefd, err := r.Open(r.output, np.OWRITE)
	if err != nil {
		log.Fatal("%v: Open error: ", db.GetName(), err)
	}
	defer r.Close(pipefd)
	fd, err := r.Open(r.input, np.OREAD)
	if err != nil {
		return "File not found"
	}
	defer r.Close(fd)
	for {
		data, err := r.Read(fd, memfs.PIPESZ)
		if len(data) == 0 || err != nil {
			break
		}
		_, err = r.Write(pipefd, data)
		if err != nil {
			log.Fatal("%v: Error pipe Write: %v", db.GetName(), err)
		}
	}
	return "OK"
}

func (r *Reader) Exit(status string) {
	r.Exited(proc.GetPid(), status)
}
