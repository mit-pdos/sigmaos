package main

import (
	"bufio"
	"log"
	"os"

	"ulambda/fs"
	"ulambda/fsclnt"
	"ulambda/fsd"
	"ulambda/fssrv"
	np "ulambda/ninep"
)

const (
	Stdin  = fs.RootInum + 1
	Stdout = fs.RootInum + 2
)

type Consoled struct {
	clnt *fsclnt.FsClient
	srv  *fssrv.FsServer
	fsd  *fsd.Fsd
	done chan bool
}

func makeConsoled() *Consoled {
	cons := &Consoled{}
	cons.clnt = fsclnt.MakeFsClient()
	cons.fsd = fsd.MakeFsd()
	cons.srv = fssrv.MakeFsServer(cons.fsd, ":0")
	cons.done = make(chan bool)
	return cons
}

//func (cons *Consoled) Connect(conn net.Conn) fssrv.FsClient {
//	return cons
//}

type Console struct {
	stdin  *bufio.Reader
	stdout *bufio.Writer
}

func makeConsole() *Console {
	cons := &Console{}
	cons.stdin = bufio.NewReader(os.Stdin)
	cons.stdout = bufio.NewWriter(os.Stdout)
	return cons

}

func (cons *Console) Write(data []byte) (np.Tsize, error) {
	n, err := cons.stdout.Write(data)
	cons.stdout.Flush()
	return np.Tsize(n), err
}

func (cons *Console) Read(n np.Tsize) ([]byte, error) {
	b, err := cons.stdin.ReadBytes('\n')
	return b, err
}

func (cons *Consoled) FsInit() {
	fs := cons.fsd.Root()
	_, err := fs.MkNod(fs.RootInode(), "console", makeConsole())
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
		name := cons.srv.MyAddr()
		err = cons.clnt.Symlink(name+":pubkey:console", "name/consoled")
		if err != nil {
			log.Fatal("Symlink error: ", err)
		}
		// XXX for testing...
		fd1, err := cons.clnt.Open("name/consoled/console", np.OWRITE)
		if err != nil {
			log.Fatal("Open error: ", err)
		}
		_, err = cons.clnt.Write(fd1, 0, []byte("Hello world\n"))
		if err != nil {
			log.Fatal("Write error: ", err)
		}
		err = cons.clnt.Close(fd1)
		if err != nil {
			log.Fatal("Close error: ", err)
		}
	} else {
		log.Fatal("Open error: ", err)
	}
	<-cons.done
	// cons.clnt.Close(fd)
	log.Printf("Consoled: finished\n")
}
