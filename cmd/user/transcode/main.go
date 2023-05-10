package main

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"

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
	s := t.Work()
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
	wrt, err := t.CreateWriter(t.output, 0777, sp.OWRITE)
	if err != nil {
		db.DFatalf("%v: Open error: %v", proc.GetProgram(), err)
	}
	defer wrt.Close()

	my_image, err := jpeg.Decode(rdr)
	if err != nil {
		return proc.MakeStatusErr("Decode", err)
	}
	my_sub_image := my_image.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(0, 0, 10, 10))
	jpeg.Encode(wrt, my_sub_image, nil)

	return proc.MakeStatus(proc.StatusOK)
}
