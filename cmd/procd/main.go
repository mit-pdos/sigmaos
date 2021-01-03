package main

import (
	//"encoding/json"
	"errors"
	//"fmt"
	"log"
	"os"
	"os/exec"
	//"strconv"
	//"strings"

	"ulambda/fsclnt"
	"ulambda/fsd"
	"ulambda/fssrv"
	np "ulambda/ninep"
	"ulambda/proc"
)

// XXX maybe one per machine, all mounting on /proc. implement union mount

type Proc struct {
	procd *Procd
	pid   string
	cmd   *exec.Cmd
}

func makeProc(p *Procd) *Proc {
	log.Printf("makeProc: pid %v\n", "")
	return &Proc{p, "", nil}
}

func (p *Proc) Write(data []byte) (int, error) {
	if string(data) == "start" {
		// program, err := p.procd.getAttr(start, p.pid, "program")
		// if err != nil {
		// 	return 0, err
		// }
		// b, err := p.procd.getAttr(start, p.pid, "args")
		// if err != nil {
		// 	return 0, err
		// }
		// var args []string
		// err = json.Unmarshal(b, &args)
		// if err != nil {
		// 	return 0, errors.New("Bad args")
		// }
		// log.Printf("args %v\n", args)
		// b, err = p.procd.getAttr(start, p.pid, "fds")
		// if err != nil {
		// 	return 0, err
		// }
		// var fids []string
		// err = json.Unmarshal(b, &fids)
		// if err != nil {
		// 	return 0, errors.New("Bad fids")
		// }
		// log.Printf("fids %v\n", fids)

		// l := strconv.Itoa(len(args))
		// a := append([]string{l}, args...)
		// a = append(a, fids...)

		// log.Printf("command %v %v\n", string(program), a)

		// p.cmd = exec.Command("./bin/"+string(program), a...)
		// p.cmd.Stdout = os.Stdout
		// p.cmd.Stderr = os.Stderr
		// err = p.cmd.Start()
		// if err != nil {
		// 	return -1, fmt.Errorf("Exec failure %v", err)
		// }
		return 0, nil
	} else {
		return 0, errors.New("Unknown command")
	}
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
	_, err := fs.MkNod(fs.RootInode(), "makeproc", makeProc(p))
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
