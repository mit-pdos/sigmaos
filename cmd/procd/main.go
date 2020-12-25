package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"ulambda/fs"
	"ulambda/fsrpc"
	"ulambda/name"
	"ulambda/proc"
)

type Proc struct {
	program string
}

type Procd struct {
	mu      sync.Mutex
	nextpid int
	procs   map[int]*Proc
	clnt    *fs.FsClient
	srv     *name.Root
	done    chan bool
}

func (p *Procd) Walk(path string) (*fsrpc.Ufd, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ufd, rest, err := p.srv.Walk(path)
	return ufd, rest, err
}

func (p *Procd) Open(ufd *fsrpc.Ufd) (fsrpc.Fd, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	inode, err := p.srv.Open(ufd)
	fd := fsrpc.Fd(inode.Inum)
	return fd, err
}

func (p *Procd) Create(path string) (fsrpc.Fd, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.nextpid += 1
	log.Printf("Create %v %d\n", path, p.nextpid)
	if path == "spawn" {
		err := p.srv.Create(strconv.Itoa(p.nextpid),
			name.InodeNumber(p.nextpid), &Proc{""})
		if err == nil {
			return fsrpc.Fd(p.nextpid), nil
		} else {
			return fsrpc.Fd(0), err
		}
	} else {
		return fsrpc.Fd(0), errors.New("Unsupported")
	}
}

func (p *Procd) Write(fd fsrpc.Fd, buf []byte) (int, error) {
	args := strings.Split(string(buf), " ")
	log.Printf("Write %v %v\n", fd, args)
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
	i, err := p.srv.Fd2Inode(fd)
	proc := i.Data.(*Proc)
	proc.program = args[0]
	return 0, nil
}

// XXX return pid if dir?
func (p *Procd) Read(fd fsrpc.Fd, n int) ([]byte, error) {
	return nil, errors.New("Unsupported")
}

func (p *Procd) Mount(fd *fsrpc.Ufd, path string) error {
	return errors.New("Unsupported")
}

func mkProc(program string) *Proc {
	p := &Proc{program}
	return p
}

// XXX what should close() mean?
func pinit(clnt *fs.FsClient, program string) {
	if _, err := clnt.Open("/console/stdin"); err != nil {
		log.Fatal("Open error stdin: ", err)
	}
	if _, err := clnt.Open("/console/stdout"); err != nil {
		log.Fatal("Open error stdout: ", err)
	}
	fds := clnt.Lsof()
	err := proc.Spawn(clnt, program, fds[1:])
	if err != nil {
		log.Fatal("Spawn error: ", err)
	}
	log.Printf("start %v\n", program)
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("missing argument")
	}
	p := &Procd{sync.Mutex{}, int(name.RootInum), make(map[int]*Proc), nil, nil,
		make(chan bool)}
	p.clnt, p.srv = fs.MakeFs(p, false)
	if fd, err := p.clnt.Open("."); err == nil {
		log.Printf("opened proc")
		err := p.clnt.Mount(fd, "/proc")
		if err != nil {
			log.Fatal("Mount error:", err)
		}
	} else {
		log.Fatal("Open error proc: ", err)
	}
	pinit(p.clnt, os.Args[1])
	<-p.done
	log.Printf("Procd: finished %v\n")
}
