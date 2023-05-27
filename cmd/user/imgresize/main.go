package main

import (
	"errors"
	"fmt"
	"image/jpeg"
	"log"
	"os"
	"time"

	"github.com/nfnt/resize"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Crop picture <in> to <out>
//

func main() {
	t, err := MakeTrans(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	start := time.Now()
	s := t.Work()
	db.DPrintf(db.ALWAYS, "Time e2e resize: %v", time.Since(start))
	t.Exited(s)
}

type Trans struct {
	*sigmaclnt.SigmaClnt
	input  string
	output string
	ctx    fs.CtxI
}

func MakeTrans(args []string) (*Trans, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeReader: too few arguments")
	}
	log.Printf("MakeTrans %v: %v\n", proc.GetPid(), args)
	t := &Trans{}
	sc, err := sigmaclnt.MkSigmaClnt("fsreader")
	if err != nil {
		return nil, err
	}
	t.SigmaClnt = sc
	t.input = args[1]
	t.output = args[2]
	t.Started()
	return t, nil
}

func (t *Trans) Work() *proc.Status {
	rdr, err := t.OpenReader(t.input)
	if err != nil {
		return proc.MakeStatusErr("File not found", err)
	}
	defer rdr.Close()

	ds := time.Now()
	img, err := jpeg.Decode(rdr)
	if err != nil {
		return proc.MakeStatusErr("Decode", err)
	}
	db.DPrintf(db.ALWAYS, "Time read/decode: %v", time.Since(ds))
	dr := time.Now()
	img1 := resize.Resize(160, 0, img, resize.Lanczos3)
	db.DPrintf(db.ALWAYS, "Time resize: %v", time.Since(dr))

	wrt, err := t.CreateWriter(t.output, 0777, sp.OWRITE)
	if err != nil {
		db.DFatalf("%v: Open %v error: %v", proc.GetProgram(), t.output, err)
	}
	defer wrt.Close()

	dw := time.Now()
	jpeg.Encode(wrt, img1, nil)
	db.DPrintf(db.ALWAYS, "Time write/encode: %v", time.Since(dw))
	return proc.MakeStatus(proc.StatusOK)
}
