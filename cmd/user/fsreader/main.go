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
	r.FsLib = fslib.MakeFsLib("fsreader")
	r.ProcClnt = procclnt.MakeProcClnt(r.FsLib)
	r.input = args[2]
	r.output = path.Join(proc.PARENTDIR, proc.SHARED) + "/"
	r.Started()
	return r, nil
}

func (r *Reader) Work() *proc.Status {
	db.DLPrintf("Reader", "Reader: work\n")
	// Open the pipe.
	pipefd, err := r.Open(r.output, np.OWRITE)
	if err != nil {
		log.Fatalf("%v: Open error: %v", proc.GetProgram(), err)
	}
	defer r.Close(pipefd)
	fd, err := r.Open(r.input, np.OREAD)
	if err != nil {
		return proc.MakeStatusErr("File not found", nil)
	}
	defer r.Close(fd)
	for {
		data, err := r.Read(fd, memfs.PIPESZ)
		if len(data) == 0 || err != nil {
			break
		}
		_, err = r.Write(pipefd, data)
		if err != nil {
			log.Fatalf("%v: Error pipe Write: %v", proc.GetProgram(), err)
		}
	}
	return proc.MakeStatus(proc.StatusOK)
}

func (r *Reader) Exit(status *proc.Status) {
	r.Exited(status)
}
