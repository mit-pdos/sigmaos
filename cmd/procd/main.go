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

	"ulambda/fs"
	"ulambda/fsclnt"
	"ulambda/fsd"
	"ulambda/fssrv"
	np "ulambda/ninep"
	"ulambda/proc"
)

// XXX maybe one per machine, all mounting on /proc. implement union mount

type Proc struct {
	fs *fs.Root
}

func makeProc(fs *fs.Root) *Proc {
	log.Printf("makeProc: pid %v\n", "")
	return &Proc{fs}
}

func (proc *Proc) getAttr(path string) ([]byte, error) {
	p := strings.Split(path, "/")
	inodes, _, err := proc.fs.Walk(proc.fs.RootInode(), p)
	if err != nil {
		return nil, err
	}
	i := inodes[len(inodes)-1]
	b := i.Data.([]byte)
	return b, nil
}

func (proc *Proc) Write(data []byte) (np.Tsize, error) {
	instr := strings.Split(string(data), " ")
	log.Printf("Proc: %v\n", instr)
	if instr[0] == "Start" {
		pid := instr[1]
		programb, err := proc.getAttr(pid + "/program")
		program := string(programb)
		log.Printf("program %v\n", program)
		if err != nil {
			return 0, err
		}
		b, err := proc.getAttr(pid + "/args")
		if err != nil {
			return 0, err
		}
		var args []string
		err = json.Unmarshal(b, &args)
		if err != nil {
			return 0, errors.New("Bad args")
		}
		log.Printf("args %v\n", args)
		b, err = proc.getAttr(pid + "/fds")
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

		log.Printf("command %v %v\n", program, a)

		cmd := exec.Command("./bin/"+program, a...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Start()
		if err != nil {
			return 0, fmt.Errorf("Exec failure %v", err)
		}
		return np.Tsize(len(data)), nil
	} else {
		return 0, errors.New("Unknown command")
	}
}

func (p *Proc) Read(n np.Tsize) ([]byte, error) {
	return nil, errors.New("Not readable")
}

type Procd struct {
	clnt *fsclnt.FsClient
	srv  *fssrv.FsServer
	fsd  *fsd.Fsd
	done chan bool
}

func makeProcd() *Procd {
	p := &Procd{}
	p.clnt = fsclnt.MakeFsClient()
	p.fsd = fsd.MakeFsd()
	p.srv = fssrv.MakeFsServer(p.fsd, ":0")
	p.done = make(chan bool)
	return p
}

func (p *Procd) FsInit() {
	fs := p.fsd.Root()
	_, err := fs.MkNod(fs.RootInode(), "makeproc", makeProc(fs))
	if err != nil {
		log.Fatal("FsInit mknod error: ", err)
	}
}

// XXX what should close() mean?
func pinit(clnt *fsclnt.FsClient, program string) {
	if _, err := clnt.Open("name/consoled/console", np.ORDWR); err != nil {
		log.Fatal("Open error console: ", err)
	}
	fds := clnt.Lsof()
	_, err := proc.Spawn(clnt, program, []string{}, fds)
	if err != nil {
		log.Fatal("Spawn error: ", err)
	}
	log.Printf("start %v\n", program)
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("missing argument")
	}
	proc := makeProcd()
	proc.FsInit()
	if fd, err := proc.clnt.Attach(":1111", ""); err == nil {
		err := proc.clnt.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
		name := proc.srv.MyAddr()
		err = proc.clnt.Symlink(name+":pubkey:proc", "name/procd")
		if err != nil {
			log.Fatal("Symlink error: ", err)
		}
	} else {
		log.Fatal("Attach error proc: ", err)
	}
	pinit(proc.clnt, os.Args[1])
	<-proc.done
	log.Printf("Procd: finished %v\n")
}
