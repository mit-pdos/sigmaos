package main

import (
	"bufio"
	"errors"
	"log"
	"os"
	"sync"

	"ulambda/fid"
	"ulambda/fs"
	"ulambda/name"
)

const (
	Stdin  = fid.RootId + 1
	Stdout = fid.RootId + 2
)

type Consoled struct {
	mu     sync.Mutex
	stdin  *bufio.Reader
	stdout *bufio.Writer
	clnt   *fs.FsClient
	srv    *name.Root
	done   chan bool
}

func makeConsoled() *Consoled {
	cons := &Consoled{}
	cons.stdin = bufio.NewReader(os.Stdin)
	cons.stdout = bufio.NewWriter(os.Stdout)
	cons.done = make(chan bool)
	return cons
}

func (cons *Consoled) Walk(start fid.Fid, path string) (*fid.Ufid, string, error) {
	cons.mu.Lock()
	defer cons.mu.Unlock()

	ufd, rest, err := cons.srv.Walk(start, path)
	return ufd, rest, err
}

func (cons *Consoled) Open(fid fid.Fid) error {
	cons.mu.Lock()
	defer cons.mu.Unlock()

	_, err := cons.srv.OpenFid(fid)
	return err
}

func (cons *Consoled) Create(f fid.Fid, t fid.IType, path string) (fid.Fid, error) {
	return fid.NullFid(), errors.New("Unsupported")
}

func (cons *Consoled) Symlink(fid fid.Fid, src string, start *fid.Ufid, dst string) error {
	return errors.New("Unsupported")
}

func (cons *Consoled) Pipe(fid fid.Fid, name string) error {
	return errors.New("Unsupported")
}

func (cons *Consoled) Write(fid fid.Fid, buf []byte) (int, error) {
	if fid.Id != Stdout {
		return 0, errors.New("Cannot write to this fd")
	}

	n, err := cons.stdout.Write(buf)
	cons.stdout.Flush()
	return n, err
}

func (cons *Consoled) Read(fid fid.Fid, n int) ([]byte, error) {
	if fid.Id != Stdin {
		return nil, errors.New("Cannot read from this fd")
	}

	b, err := cons.stdin.ReadBytes('\n')
	return b, err
}

func (cons *Consoled) Mount(fd *fid.Ufid, start fid.Fid, path string) error {
	return errors.New("Unsupported")
}

func (cons *Consoled) FsInit() {
	rfid := cons.srv.RootFid()
	_, err := cons.srv.Create(rfid, "stdin")
	if err != nil {
		log.Fatal("Create error: ", err)
	}
	_, err = cons.srv.Create(rfid, "stdout")
	if err != nil {
		log.Fatal("Create error: ", err)
	}
}

func main() {
	cons := makeConsoled()
	cons.clnt, cons.srv = fs.MakeFs(cons, false)
	cons.FsInit()
	if fd, err := cons.clnt.Open("."); err == nil {
		err := cons.clnt.Mount(fd, "/console")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
	} else {
		log.Fatal("Open error:", err)
	}
	<-cons.done
	log.Printf("Consoled: finished\n")
}
