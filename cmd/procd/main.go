package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"ulambda/fsclnt"
	"ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

// XXX maybe one per machine, all mounting on /proc. implement union mount

type Proc struct {
	fs *memfs.Root
}

func (proc *Proc) Len() np.Tlength {
	return 0
}

func makeProc(fs *memfs.Root) *Proc {
	log.Printf("makeProc: pid %v\n", "")
	return &Proc{fs}
}

func (proc *Proc) getAttr(path string) ([]byte, error) {
	p := strings.Split(path, "/")
	root := proc.fs.RootInode()
	inodes, _, err := root.Walk(p)
	if err != nil {
		return nil, err
	}
	i := inodes[len(inodes)-1]
	return i.Read(0, 1024)
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
		go func() {
			cmd.Wait()
			log.Printf("command %v exited\n", program)
		}()
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
	srv  *npsrv.NpServer
	fsd  *memfsd.Fsd
	done chan bool
}

func makeProcd() *Procd {
	p := &Procd{}
	p.clnt = fsclnt.MakeFsClient("name/procd/init")
	p.fsd = memfsd.MakeFsd()
	p.srv = npsrv.MakeNpServer(p.fsd, ":0")
	p.done = make(chan bool)
	return p
}

func (p *Procd) FsInit() {
	fs := p.fsd.Root()
	rooti := fs.RootInode()
	_, err := fs.MkNod(rooti, "makeproc", makeProc(fs))
	if err != nil {
		log.Fatal("FsInit mknod error: ", err)
	}
	pid := filepath.Base(p.clnt.Proc)
	_, err = rooti.Create(fs, np.DMDIR|0777, pid)
	if err != nil {
		log.Fatal("FsInit mkdir error: ", err)
	}
}

func main() {
	proc := makeProcd()
	proc.FsInit()
	if fd, err := proc.clnt.Attach(":1111", ""); err == nil {
		err := proc.clnt.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
		name := proc.srv.MyAddr()
		err = proc.clnt.Symlink(name+":pubkey:proc", "name/procd", 0700)
		if err != nil {
			log.Fatal("Symlink error: ", err)
		}
	} else {
		log.Fatal("Attach error proc: ", err)
	}
	<-proc.done
	log.Printf("Procd: finished %v\n")
}
