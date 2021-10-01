package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/fsclnt"
	"ulambda/fslibsrv"
	"ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
)

//
// Creates a named pipe in name/<name>/pipe (name is os.Args[2]), reads a
// data from input (os.Args[3])), and writes it to the named pipe.
//

func main() {
	m, err := MakeReader(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	s := m.Work()
	m.Exit(s)
}

type Reader struct {
	*fslibsrv.FsLibSrv
	proc.ProcClnt
	pid   string
	input string
	pipe  fs.FsObj
}

func MakeReader(args []string) (*Reader, error) {
	if len(args) != 4 {
		return nil, errors.New("MakeReader: too few arguments")
	}
	log.Printf("MakeReader: %v\n", args)

	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, errors.New("MakeReader: No IP")
	}
	n := "name/" + args[2]
	memfsd := memfsd.MakeFsd(ip + ":0")
	pipe, err := memfsd.MkPipe("pipe")
	if err != nil {
		log.Fatal("Create error: ", err)
	}

	fsl, err := fslibsrv.InitFs(n, memfsd)
	if err != nil {
		return nil, err
	}

	r := &Reader{}
	r.FsLibSrv = fsl
	r.ProcClnt = procinit.MakeProcClnt(fsl.FsLib, procinit.GetProcLayersMap())
	r.pid = args[1]
	r.input = args[3]
	r.pipe = pipe
	r.Started(r.pid)

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
	r.ExitFs("name/" + r.pid)
	r.Exited(r.pid, status)
}
