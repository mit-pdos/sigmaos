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
// data from input (os.Args[1])), and writes it to the named pipe.
//

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid args...\n", os.Args[0])
		os.Exit(1)
	}
	m, err := MakeReader(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	m.Work()
	m.Exit()
}

type Reader struct {
	*fslibsrv.FsLibSrv
	proc.ProcClnt
	pid   string
	input string
	pipe  fs.FsObj
}

func MakeReader(args []string) (*Reader, error) {
	if len(args) != 3 {
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

	fsl, err := fslibsrv.InitFs(n, memfsd, nil)
	if err != nil {
		return nil, err
	}

	r := &Reader{}
	r.FsLibSrv = fsl
	r.ProcClnt = procinit.MakeProcClnt(fsl.FsLib, procinit.GetProcLayersMap())
	r.pid = args[0]
	r.input = args[1]
	r.pipe = pipe
	r.Started(r.pid)

	return r, nil
}

func (r *Reader) Work() {
	db.DLPrintf("Reader", "Reader: work\n")
	_, err := r.pipe.Open(nil, np.OWRITE)
	if err != nil {
		log.Fatal("Open error: ", err)
	}
	fd, err := r.Open(r.input, np.OREAD)
	if err != nil {
		// XXX need to return an sigmaOS exit value so that
		// wwwd terminates waiting for this process
		os.Exit(66) //EX_NOINPUT
	}
	for {
		data, err := r.Read(fd, memfs.PIPESZ)
		if len(data) == 0 {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		_, err = r.pipe.(fs.File).Write(nil, 0, data, np.NoV)
		if err != nil {
			log.Fatal(err)
		}
	}
	r.Close(fd)
	r.pipe.Close(nil, np.OWRITE)

	r.ExitFs("name/" + r.pid)
}

func (r *Reader) Exit() {
	r.Exited(r.pid, "OK")
}
