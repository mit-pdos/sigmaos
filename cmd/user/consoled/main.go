package main

import (
	"bufio"
	"log"
	"os"

	"ulambda/dir"
	"ulambda/fs"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	np "ulambda/ninep"
)

type Consoled struct {
	*fssrv.FsServer
}

func makeConsoled() *Consoled {
	cons := &Consoled{}
	root, srv, _, _, err := fslibsrv.MakeMemFs("name/consoled", "consoled")
	if err != nil {
		log.Fatalf("MakeSrvFsLib %v\n", err)
	}
	cons.FsServer = srv
	err = dir.MkNod(fssrv.MkCtx(""), root, "console", makeConsole())
	if err != nil {
		log.Fatalf("MakeNod failed %v\n", err)
	}
	return cons
}

type Console struct {
	fs.FsObj
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
	cons.Serve()
	log.Printf("Consoled: finished\n")
}
