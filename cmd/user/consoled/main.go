package main

import (
	"bufio"
	"log"
	"os"

	"ulambda/dir"
	"ulambda/fs"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/inode"
	"ulambda/memfsd"
	"ulambda/netsrv"
	np "ulambda/ninep"
)

type Consoled struct {
	*fslibsrv.FsLibSrv
	srv    *netsrv.NetServer
	memfsd *memfsd.Fsd
	done   chan bool
}

func makeConsoled() *Consoled {
	cons := &Consoled{}
	fsd := memfsd.MakeFsd(":0")
	fsl, err := fslibsrv.InitFs("name/consoled", fsd)
	if err != nil {
		log.Fatalf("InitFs: err %v\n", err)
	}
	err = dir.MkNod(fssrv.MkCtx(""), fsd.GetRoot(), "console", makeConsole(fsd.GetRoot()))
	if err != nil {
		log.Fatalf("MakeNod failed %v\n", err)
	}
	cons.FsLibSrv = fsl
	cons.done = make(chan bool)
	return cons
}

type Console struct {
	fs.FsObj
	stdin  *bufio.Reader
	stdout *bufio.Writer
}

func makeConsole(parent fs.Dir) *Console {
	cons := &Console{}
	cons.FsObj = inode.MakeInode("", np.DMDEVICE, parent)
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
