package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"ulambda/fs"
	"ulambda/fsrpc"
	"ulambda/name"
	"ulambda/proc"
)

// XXX maybe one per machine, all mounting on /proc. implement union mount

type Proc struct {
	procd *Procd
	start fsrpc.Fid
	pid   string
	cmd   *exec.Cmd
}

func makeProc(p *Procd, start fsrpc.Fid, pid string) *Proc {
	log.Printf("makeProc: start %v pid %v\n", pid)
	return &Proc{p, start, pid, nil}
}

func (p *Proc) Write(fid fsrpc.Fid, data []byte) (int, error) {
	if string(data) == "start" {
		program, err := p.procd.getAttr(p.start, p.pid, "/program")
		if err != nil {
			return 0, err
		}
		b, err := p.procd.getAttr(p.start, p.pid, "/fds")
		if err != nil {
			return 0, err
		}
		var fids []string
		err = json.Unmarshal(b, &fids)
		if err != nil {
			return 0, errors.New("Bad fids")
		}
		log.Printf("command %v %v\n", string(program), fids)
		p.cmd = exec.Command(string(program), fids...)
		p.cmd.Stdout = os.Stdout
		p.cmd.Stderr = os.Stderr
		err = p.cmd.Start()
		if err != nil {
			return -1, fmt.Errorf("Exec failure %v", err)
		}
		return 0, nil
	} else {
		return 0, errors.New("Unknown command")
	}
}

func (p *Proc) Read(fid fsrpc.Fid, n int) ([]byte, error) {
	return nil, errors.New("Unsupported")
}

type Procd struct {
	mu   sync.Mutex
	clnt *fs.FsClient
	srv  *name.Root
	done chan bool
}

// XXX close
func (p *Procd) getAttr(fid fsrpc.Fid, pid string, key string) ([]byte, error) {
	inode, err := p.srv.Open(fid, pid+"/"+key)
	if err != nil {
		return nil, err
	}
	b, err := p.srv.Read(inode.Fid, 1024)
	return b, err
}

func (p *Procd) Walk(start fsrpc.Fid, path string) (*fsrpc.Ufid, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ufd, rest, err := p.srv.Walk(start, path)
	return ufd, rest, err
}

func (p *Procd) Open(fid fsrpc.Fid, path string) (fsrpc.Fid, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Printf("Procd open %v %v\n", fid, path)
	inode, err := p.srv.Open(fid, path)
	if err != nil {
		return fsrpc.NullFid(), err
	}
	return inode.Fid, err
}

func (p *Procd) Create(fid fsrpc.Fid, path string) (fsrpc.Fid, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Printf("Procd create %v\n", path)
	diri, err := p.srv.MkDir(fid, path)
	if err != nil {
		return fsrpc.NullFid(), err
	}
	_, err = p.srv.Create(fid, path+"/program")
	if err != nil {
		return fsrpc.NullFid(), err
	}
	_, err = p.srv.Create(fid, path+"/fds")
	if err != nil {
		return fsrpc.NullFid(), err
	}
	err = p.srv.Pipe(fid, path+"/exit")
	if err != nil {
		return fsrpc.NullFid(), err
	}
	proc := makeProc(p, fid, path)
	err = p.srv.MkNod(fid, path+"/ctl", proc)
	if err != nil {
		return fsrpc.NullFid(), err
	}
	return diri.Fid, nil
}

func (p *Procd) Write(fid fsrpc.Fid, buf []byte) (int, error) {
	log.Printf("Write %v %v\n", fid, strings.Split(string(buf), " "))
	return p.srv.Write(fid, buf)
}

func (p *Procd) Read(fid fsrpc.Fid, n int) ([]byte, error) {
	log.Printf("Read %v %v\n", fid, n)
	return p.srv.Read(fid, n)
}

func (p *Procd) Mount(fd *fsrpc.Ufid, start fsrpc.Fid, path string) error {
	return errors.New("Unsupported")
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
	_, err := proc.Spawn(clnt, program, fds[1:])
	if err != nil {
		log.Fatal("Spawn error: ", err)
	}
	log.Printf("start %v\n", program)
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("missing argument")
	}
	p := &Procd{sync.Mutex{}, nil, nil, make(chan bool)}
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
