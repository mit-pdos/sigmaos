package main

import (
	"bufio"
	//"errors"
	"log"
	"net"
	"os"
	"sync"

	"ulambda/fs"
	"ulambda/fsclnt"
	"ulambda/fssrv"

	np "ulambda/ninep"
)

const (
	Stdin  = fs.RootInum + 1
	Stdout = fs.RootInum + 2
)

type Consoled struct {
	mu     sync.Mutex
	stdin  *bufio.Reader
	stdout *bufio.Writer
	clnt   *fsclnt.FsClient
	srv    *fssrv.FsServer
	fs     *fs.Root
	done   chan bool
}

func makeConsoled() *Consoled {
	cons := &Consoled{}
	cons.stdin = bufio.NewReader(os.Stdin)
	cons.stdout = bufio.NewWriter(os.Stdout)
	cons.clnt = fsclnt.MakeFsClient()
	cons.srv = fssrv.MakeFsServer(cons, ":0")
	cons.fs = fs.MakeRoot()
	cons.done = make(chan bool)
	return cons
}

func (cons *Consoled) Connect(conn net.Conn) fssrv.FsClient {
	return cons
}

// func (cons *Consoled) Write(args np.Twrite, reply *np.Rwrite) error {
// 	if fid.Id != Stdout {
// 		return 0, errors.New("Cannot write to this fd")
// 	}

// 	n, err := cons.stdout.Write(buf)
// 	cons.stdout.Flush()
// 	return n, err
// }

// func (cons *Consoled) Read(args np.Tread, reply *np.Rread) error {
// 	if fid.Id != Stdin {
// 		return nil, errors.New("Cannot read from this fd")
// 	}

// 	b, err := cons.stdin.ReadBytes('\n')
// 	return b, err
// }

func (cons *Consoled) FsInit() {
	root := cons.fs.RootInode()
	_, err := cons.fs.Create(root, "stdin", np.DMAPPEND)
	if err != nil {
		log.Fatal("Create error: ", err)
	}
	_, err = cons.fs.Create(root, "stdout", np.DMAPPEND)
	if err != nil {
		log.Fatal("Create error: ", err)
	}
}

func main() {
	cons := makeConsoled()
	cons.FsInit()
	if fd, err := cons.clnt.Attach(":1111", ""); err == nil {
		err := cons.clnt.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
		_, err = cons.clnt.Create("name/x", 0, np.OWRITE)
		if err != nil {
			log.Fatal("Create error: ", err)
		}
	} else {
		log.Fatal("Open error: ", err)
	}
	<-cons.done
	// cons.clnt.Close(fd)
	log.Printf("Consoled: finished\n")
}
