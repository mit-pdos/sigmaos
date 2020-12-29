package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"ulambda/fid"
	"ulambda/fs"
	"ulambda/name"
	"ulambda/proc"
)

// XXX maybe one per machine, all mounting on /proc. implement union mount

type Proc struct {
	procd *Procd
	start fid.Fid
	pid   string
	cmd   *exec.Cmd
}

func makeProc(p *Procd, start fid.Fid, pid string) *Proc {
	log.Printf("makeProc: start %v pid %v\n", start, pid)
	return &Proc{p, start, pid, nil}
}

func (p *Proc) Write(fid fid.Fid, data []byte) (int, error) {
	if string(data) == "start" {
		program, err := p.procd.getAttr(p.start, p.pid, "program")
		if err != nil {
			return 0, err
		}
		b, err := p.procd.getAttr(p.start, p.pid, "args")
		if err != nil {
			return 0, err
		}
		var args []string
		err = json.Unmarshal(b, &args)
		if err != nil {
			return 0, errors.New("Bad args")
		}
		log.Printf("args %v\n", args)
		b, err = p.procd.getAttr(p.start, p.pid, "fds")
		if err != nil {
			return 0, err
		}
		var fids []string
		err = json.Unmarshal(b, &fids)
		if err != nil {
			return 0, errors.New("Bad fids")
		}
		log.Printf("fids %v\n", fids)

		l := strconv.Itoa(len(args))
		a := append([]string{l}, args...)
		a = append(a, fids...)

		log.Printf("command %v %v\n", string(program), a)

		p.cmd = exec.Command(string(program), a...)
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

func (p *Proc) Read(fid fid.Fid, n int) ([]byte, error) {
	return nil, errors.New("Unsupported")
}

type Procd struct {
	mu   sync.Mutex
	clnt *fs.FsClient
	srv  *name.Root
	done chan bool
}

// XXX close
func (p *Procd) getAttr(fid fid.Fid, pid string, key string) ([]byte, error) {
	inode, err := p.srv.WalkOpenFid(fid, pid+"/"+key)
	if err != nil {
		return nil, err
	}
	return p.srv.Read(inode.Fid, 1024)
}

func (p *Procd) Walk(start fid.Fid, path string) (*fid.Ufid, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ufd, rest, err := p.srv.Walk(start, path)
	return ufd, rest, err
}

func (p *Procd) Open(fid fid.Fid) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Printf("Procd open %v\n", fid)
	_, err := p.srv.OpenFid(fid)
	return err
}

func (p *Procd) Create(f fid.Fid, t fid.IType, path string) (fid.Fid, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Printf("Procd create %v %v\n", path, t)
	if t == fid.DirT {
		i, err := p.srv.MkDir(f, path)
		if err != nil {
			return fid.NullFid(), err
		}
		proc := makeProc(p, f, path)
		err = p.srv.MkNod(f, path+"/ctl", proc)
		if err != nil {
			return fid.NullFid(), err
		}
		return i.Fid, nil
	} else {
		i, err := p.srv.Create(f, path)
		if err != nil {
			return fid.NullFid(), err
		}
		return i.Fid, nil
	}
}

func (p *Procd) Symlink(f fid.Fid, src string, start *fid.Ufid, dst string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, err := p.srv.Symlink(f, src, start, dst)
	return err
}

func (p *Procd) Pipe(f fid.Fid, name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.srv.Pipe(f, name)
}

func (p *Procd) Write(f fid.Fid, buf []byte) (int, error) {
	log.Printf("Write %v %v\n", f, strings.Split(string(buf), " "))
	return p.srv.Write(f, buf)
}

func (p *Procd) Read(f fid.Fid, n int) ([]byte, error) {
	log.Printf("Read %v %v\n", f, n)
	return p.srv.Read(f, n)
}

func (p *Procd) Mount(uf *fid.Ufid, start fid.Fid, path string) error {
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
	_, err := proc.Spawn(clnt, program, []string{}, fds[1:])
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
