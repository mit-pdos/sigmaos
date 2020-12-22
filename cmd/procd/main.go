package main

import (
	"log"
	"os"
	// "os/exec"
	"ulambda/fs"
	"ulambda/fsrpc"
)

type Proc struct {
	program string
}

type Procd struct {
	names map[int]*Proc
	done  chan bool
}

// open ctl
func (p *Procd) Open(path string) (*fsrpc.Fd, error) {
	return nil, nil
}

func (p *Procd) Write(buf []byte) (int, error) {
	return 0, nil
}

// if someone opens create a reader for it
func (p *Procd) Read(n int) ([]byte, error) {
	return nil, nil
}

func (p *Procd) Mount(fd *fsrpc.Fd, path string) error {
	return nil
}

func mkProc(program string) *Proc {
	p := &Proc{program}
	return p
}

func pinit(clnt *fs.FsClient, program string) {
	if in, err := clnt.Open("/console"); err != nil {
		log.Fatal("Open error:", err)
	}
	if out, err := clnt.Open("/console"); err != nil {
		log.Fatal("Open error:", err)
	}
	p, err := proc.Spawn(program, clnt)
	if err != nil {
		log.Fatal("Spawn error: ", err)
	}
	log.Printf("start %v\n", p)
	//cmd := exec.Command(os.Args[1])
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	//err = cmd.Start()
	//err = cmd.Wait()

}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("missing argument")
	}

	p := &Procd{make(map[int]*Proc), make(chan bool)}
	clnt := fs.MakeFsClient(fs.MakeFsRoot())
	err := clnt.MkNod("proc", p)
	if err != nil {
		log.Fatal("MkNod error:", err)
	}
	if fd, err := clnt.Open("proc"); err == nil {
		err := clnt.Mount(fd, "/proc")
		if err != nil {
			log.Fatal("Mount error:", err)
		}
	} else {
		log.Fatal("Open error:", err)
	}
	pinit(clnt, os.Args[1])
	<-p.done
	log.Printf("Procd: finished %v\n", err)
}
