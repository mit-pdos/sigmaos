package main

import (
	"errors"
	"fmt"
	"os"
	"path"

	db "sigmaos/debug"
	"sigmaos/api/fs"
	"sigmaos/sigmasrv/pipe"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Reads data from input (os.Args[2])), and writes it to the named pipe found
// at proc.PARENTDIR/proc.SHARED
//

func main() {
	r, err := NewReader(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	s := r.Work()
	r.Exit(s)
}

type Reader struct {
	*sigmaclnt.SigmaClnt
	input  string
	output string
	ctx    fs.CtxI
}

func NewReader(args []string) (*Reader, error) {
	if len(args) != 2 {
		return nil, errors.New("NewReader: too few arguments")
	}
	pe := proc.GetProcEnv()
	db.DPrintf(db.ALWAYS, "NewReader %v: %v\n", pe.GetPID(), args)
	r := &Reader{}
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return nil, err
	}
	r.SigmaClnt = sc
	r.input = args[1]
	r.output = path.Join(proc.PARENTDIR /*, proc.SHARED*/) + "/"
	r.Started()
	return r, nil
}

func (r *Reader) Work() *proc.Status {
	db.DPrintf(db.FSREADER, "Reader: %v\n", r.input)
	// Open the pipe.
	pipefd, err := r.Open(r.output, sp.OWRITE)
	if err != nil {
		db.DFatalf("Open error: %v", err)
	}
	defer r.CloseFd(pipefd)
	fd, err := r.Open(r.input, sp.OREAD)
	if err != nil {
		return proc.NewStatusErr("File not found", nil)
	}
	defer r.CloseFd(fd)
	for {
		data := make([]byte, pipe.PIPESZ)
		cnt, err := r.Read(fd, data)
		if cnt == 0 || err != nil {
			break
		}
		_, err = r.Write(pipefd, data)
		if err != nil {
			db.DFatalf("Error pipe Write: %v", err)
		}
	}
	return proc.NewStatus(proc.StatusOK)
}

func (r *Reader) Exit(status *proc.Status) {
	r.Exit(status)
}
