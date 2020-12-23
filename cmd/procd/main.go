package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"ulambda/fs"
	"ulambda/fsrpc"
	"ulambda/proc"
)

type Proc struct {
	program string
}

type Procd struct {
	nextpid int
	procs   map[int]*Proc
	done    chan bool
}

func (p *Procd) Walk(path string) (*fsrpc.Ufd, error) {
	log.Print("Walk %v\n", path)
	return nil, nil
}

func (p *Procd) Open(path string) (fsrpc.Fd, error) {
	log.Print("Open %v\n", path)
	return 0, nil
}

func (p *Procd) Create(path string) (fsrpc.Fd, error) {
	p.nextpid += 1
	log.Printf("Create %v %d\n", path, p.nextpid)
	if path == "spawn" {
		p.procs[p.nextpid] = &Proc{""}
		return fsrpc.Fd(p.nextpid), nil
	} else {
		return fsrpc.Fd(0), errors.New("Cannot create")
	}
}

func (p *Procd) Write(fd fsrpc.Fd, buf []byte) (int, error) {
	args := strings.Split(string(buf), " ")
	log.Printf("Write %v\n", args)
	if len(args) == 0 {
		return -1, errors.New("No program")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return -1, fmt.Errorf("Exec failure %v", err)
	}
	p.procs[int(fd)].program = args[0]
	return 0, nil
}

// XXX return pid if dir?
func (p *Procd) Read(fd fsrpc.Fd, n int) ([]byte, error) {
	return nil, nil
}

func (p *Procd) Mount(fd *fsrpc.Ufd, path string) error {
	return nil
}

func mkProc(program string) *Proc {
	p := &Proc{program}
	return p
}

// XXX fd0 points to proc; what should close() mean?
func pinit(clnt *fs.FsClient, program string) {
	if _, err := clnt.Open("/console/stdin"); err != nil {
		log.Fatal("Open error stdin: ", err)
	}
	if _, err := clnt.Open("/console/stdout"); err != nil {
		log.Fatal("Open error stdout: ", err)
	}
	p, err := proc.Spawn(program, clnt)
	if err != nil {
		log.Fatal("Spawn error: ", err)
	}
	log.Printf("start %v\n", p)
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("missing argument")
	}

	p := &Procd{0, make(map[int]*Proc), make(chan bool)}
	clnt := fs.MakeFsClient(fs.MakeFsRoot())
	err := clnt.MkNod("proc", p)
	if err != nil {
		log.Fatal("MkNod error:", err)
	}
	if fd, err := clnt.Open("proc"); err == nil {
		log.Printf("opened proc")
		err := clnt.Mount(fd, "/proc")
		if err != nil {
			log.Fatal("Mount error:", err)
		}
	} else {
		log.Fatalf("Open error proc: ", err)
	}
	pinit(clnt, os.Args[1])
	<-p.done
	log.Printf("Procd: finished %v\n", err)
}
