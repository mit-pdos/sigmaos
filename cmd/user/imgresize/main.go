package main

import (
	"fmt"
	"image/jpeg"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nfnt/resize"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/perf"
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
		db.DFatalf("Error %v", err)
	}

	p, err := perf.MakePerf(perf.THUMBNAIL)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	defer p.Done()

	rand.Seed(time.Now().UnixNano())

	var s *proc.Status
	for i := 0; i < len(t.inputs); i++ {
		start := time.Now()
		output := t.output
		// Create a new file name for iterations > 0
		if i > 0 {
			output += strconv.Itoa(rand.Int())
		}
		s = t.Work(i, output)
		db.DPrintf(db.ALWAYS, "Time %v e2e resize[%v]: %v", os.Args, i, time.Since(start))
	}
	t.Exited(s)
}

type Trans struct {
	*sigmaclnt.SigmaClnt
	inputs []string
	output string
	ctx    fs.CtxI
}

func MakeTrans(args []string) (*Trans, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("MakeTrans: too few arguments: %v", args)
	}
	db.DPrintf(db.IMGD, "MakeTrans %v: %v\n", proc.GetPid(), args)
	t := &Trans{}
	sc, err := sigmaclnt.MkSigmaClnt("fsreader")
	if err != nil {
		return nil, err
	}
	t.SigmaClnt = sc
	t.inputs = strings.Split(args[1], ",")
	t.output = args[2]
	t.Started()
	return t, nil
}

func (t *Trans) Work(i int, output string) *proc.Status {
	do := time.Now()
	rdr, err := t.OpenReader(t.inputs[i])
	if err != nil {
		return proc.MakeStatusErr("File not found", err)
	}
	db.DPrintf(db.ALWAYS, "Time %v open: %v", t.inputs[i], time.Since(do))
	var dc time.Time
	defer func() {
		rdr.Close()
		db.DPrintf(db.ALWAYS, "Time %v close reader: %v", t.inputs[i], time.Since(dc))
	}()

	ds := time.Now()
	img, err := jpeg.Decode(rdr)
	if err != nil {
		return proc.MakeStatusErr("Decode", err)
	}
	db.DPrintf(db.ALWAYS, "Time %v read/decode: %v", t.inputs[i], time.Since(ds))
	dr := time.Now()
	img1 := resize.Resize(160, 0, img, resize.Lanczos3)
	db.DPrintf(db.ALWAYS, "Time %v resize: %v", t.inputs[i], time.Since(dr))

	dcw := time.Now()
	wrt, err := t.CreateWriter(output, 0777, sp.OWRITE)
	if err != nil {
		db.DFatalf("%v: Open %v error: %v", proc.GetProgram(), t.output, err)
	}
	db.DPrintf(db.ALWAYS, "Time %v create writer: %v", t.inputs[i], time.Since(dcw))
	dw := time.Now()
	defer func() {
		wrt.Close()
		db.DPrintf(db.ALWAYS, "Time %v write/encode: %v", t.inputs[i], time.Since(dw))
		dc = time.Now()
	}()

	jpeg.Encode(wrt, img1, nil)
	return proc.MakeStatus(proc.StatusOK)
}
