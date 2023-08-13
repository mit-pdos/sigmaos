package main

import (
	"errors"
	"image/jpeg"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/nfnt/resize"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const N_ITER = 1

//
// Crop picture <in> to <out>
//

func main() {
	t, err := MakeTrans(os.Args)
	if err != nil {
		db.DFatalf("Error %v", err)
	}

	p, err := perf.MakePerf(perf.THUMBNAIL)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	defer p.Done()

	rand.Seed(time.Now().UnixNano())

	var s *proc.Status
	for i := 0; i < N_ITER; i++ {
		start := time.Now()
		output := t.output
		// Create a new file name for iterations > 0
		if i > 0 {
			output += strconv.Itoa(rand.Int())
		}
		s = t.Work(output)
		db.DPrintf(db.ALWAYS, "Time %v e2e resize[%v]: %v", os.Args, i, time.Since(start))
	}
	t.ClntExit(s)
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
	db.DPrintf(db.IMGD, "MakeTrans %v: %v\n", proc.GetPid(), args)
	t := &Trans{}
	sc, err := sigmaclnt.NewSigmaClnt("fsreader")
	if err != nil {
		return nil, err
	}
	t.SigmaClnt = sc
	t.input = args[1]
	t.output = args[2]
	t.Started()
	return t, nil
}

func (t *Trans) Work(output string) *proc.Status {
	do := time.Now()
	rdr, err := t.OpenReader(t.input)
	if err != nil {
		return proc.MakeStatusErr("File not found", err)
	}
	db.DPrintf(db.ALWAYS, "Time %v open: %v", t.input, time.Since(do))
	var dc time.Time
	defer func() {
		rdr.Close()
		db.DPrintf(db.ALWAYS, "Time %v close reader: %v", t.input, time.Since(dc))
	}()

	ds := time.Now()
	img, err := jpeg.Decode(rdr)
	if err != nil {
		return proc.MakeStatusErr("Decode", err)
	}
	db.DPrintf(db.ALWAYS, "Time %v read/decode: %v", t.input, time.Since(ds))
	dr := time.Now()
	img1 := resize.Resize(160, 0, img, resize.Lanczos3)
	db.DPrintf(db.ALWAYS, "Time %v resize: %v", t.input, time.Since(dr))

	dcw := time.Now()
	wrt, err := t.CreateWriter(output, 0777, sp.OWRITE)
	if err != nil {
		db.DFatalf("%v: Open %v error: %v", proc.GetProgram(), t.output, err)
	}
	db.DPrintf(db.ALWAYS, "Time %v create writer: %v", t.input, time.Since(dcw))
	dw := time.Now()
	defer func() {
		wrt.Close()
		db.DPrintf(db.ALWAYS, "Time %v write/encode: %v", t.input, time.Since(dw))
		dc = time.Now()
	}()

	jpeg.Encode(wrt, img1, nil)
	return proc.MakeStatus(proc.StatusOK)
}
