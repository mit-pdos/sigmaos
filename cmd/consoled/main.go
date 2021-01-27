package main

import (
	"bufio"
	"log"
	"os"

	"ulambda/fslib"
	"ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

const (
	Stdin  = memfs.RootInum + 1
	Stdout = memfs.RootInum + 2
)

type Consoled struct {
	*fslib.FsLibSrv
	srv    *npsrv.NpServer
	memfsd *memfsd.Fsd
	done   chan bool
}

func makeConsoled() *Consoled {
	cons := &Consoled{}
	fsl, err := fslib.InitFs("name/consoled", makeConsole())
	if err != nil {
		log.Fatalf("InitFs: err %v\n", err)
	}
	cons.FsLibSrv = fsl
	cons.done = make(chan bool)
	return cons
}

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

func (cons *Console) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	n, err := cons.stdout.Write(data)
	cons.stdout.Flush()
	return np.Tsize(n), err
}

func (cons *Console) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	b, err := cons.stdin.ReadBytes('\n')
	return b, err
}

func (cons *Console) Len() np.Tlength { return 0 }

func main() {
	cons := makeConsoled()
	<-cons.done
	// cons.Close(fd)
	log.Printf("Consoled: finished\n")
}
