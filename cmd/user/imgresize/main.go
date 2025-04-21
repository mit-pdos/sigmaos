package main

import (
	"bufio"
	"fmt"
	"image/jpeg"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nfnt/resize"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
)

const (
	IMG_DIM = 160
)

//
// Crop picture <in> to <out>
//

func main() {
	pe := proc.GetProcEnv()
	db.DPrintf(db.IMGD, "imgresize %v: %v", pe.GetPID(), os.Args)
	p, err := perf.NewPerf(pe, perf.THUMBNAIL)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()

	t, err := NewTrans(pe, os.Args, p)
	if err != nil {
		db.DFatalf("Error %v", err)
	}

	rand.Seed(time.Now().UnixNano())

	var s *proc.Status
	for i := 0; i < len(t.inputs); i++ {
		start := time.Now()
		output := t.output
		// Create a new file name for iterations > 0
		output += strconv.Itoa(rand.Int())
		s = t.Work(i, output)
		db.DPrintf(db.ALWAYS, "Time %v e2e resize[%v]: %v", os.Args, i, time.Since(start))
	}
	if !t.runningInDocker {
		t.ClntExit(s)
	}
}

type Trans struct {
	*sigmaclnt.SigmaClnt
	inputs          []string
	output          string
	ctx             fs.CtxI
	nrounds         int
	runningInDocker bool
	p               *perf.Perf
}

func NewTrans(pe *proc.ProcEnv, args []string, p *perf.Perf) (*Trans, error) {
	if len(args) != 4 && len(args) != 5 {
		return nil, fmt.Errorf("NewTrans: wrong number of arguments: %v", args)
	}
	runningInDocker := len(args) == 5 && args[4] == "running-in-docker"
	if runningInDocker {
		db.DPrintf(db.ALWAYS, "Running in docker")
	}
	t := &Trans{
		p:               p,
		runningInDocker: runningInDocker,
	}
	db.DPrintf(db.ALWAYS, "E2e spawn time since spawn until main: %v", time.Since(pe.GetSpawnTime()))
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return nil, err
	}
	t.SigmaClnt = sc
	t.inputs = strings.Split(args[1], ",")
	db.DPrintf(db.ALWAYS, "Args {%v} inputs {%v} fail {%v}", args[1], t.inputs, proc.GetSigmaFail())
	t.output = t.inputs[0] + "-thumbnail"
	t.nrounds, err = strconv.Atoi(args[3])
	if err != nil {
		db.DFatalf("Err convert nrounds: %v", err)
	}
	if !t.runningInDocker {
		t.Started()
	}
	crash.FailersDefault(sc.FsLib, []crash.Tselector{crash.IMGRESIZE_CRASH})
	return t, nil
}

func (t *Trans) Work(i int, output string) *proc.Status {
	do := time.Now()

	db.DPrintf(db.ALWAYS, "Resize (%v/%v) %v", i, len(t.inputs), t.inputs[i])

	var rdr io.Reader
	var err error
	// Read from local OS instead of SigmaOS?
	localOS := strings.HasPrefix(t.inputs[i], "/tmp/")
	if localOS {
		f, err := os.Open(t.inputs[i])
		if err != nil {
			db.DFatalf("Err OpenFile: %v", err)
		}
		rdr = bufio.NewReaderSize(f, sp.BUFSZ)
	} else {
		rdr, err = t.OpenReader(t.inputs[i])
	}
	if err != nil {
		return proc.NewStatusErr(fmt.Sprintf("File %v not found kid %v err %v", t.inputs[i], t.ProcEnv().GetKernelID(), err), err)
	}
	//	prdr := perf.NewPerfReader(rdr, t.p)
	db.DPrintf(db.ALWAYS, "Time %v open: %v", t.inputs[i], time.Since(do))
	var dc time.Time
	defer func() {
		if !localOS {
			rdr.(*fslib.FileReader).Close()
		}
		db.DPrintf(db.ALWAYS, "Time %v close reader: %v", t.inputs[i], time.Since(dc))
	}()

	ds := time.Now()
	img, err := jpeg.Decode(rdr)
	if err != nil {
		return proc.NewStatusErr("Decode", err)
	}
	// img size in bytes:
	bounds := img.Bounds()
	var imgSizeB uint64 = 16 * uint64(bounds.Max.X-bounds.Min.X) * uint64(bounds.Max.Y-bounds.Min.Y)
	db.DPrintf(db.ALWAYS, "Time %v read/decode: %v", t.inputs[i], time.Since(ds))
	dr := time.Now()
	for i := 0; i < t.nrounds-1; i++ {
		resize.Resize(IMG_DIM, IMG_DIM, img, resize.Lanczos3)
		t.p.TptTick(float64(imgSizeB))
	}
	img1 := resize.Resize(IMG_DIM, IMG_DIM, img, resize.Lanczos3)
	t.p.TptTick(float64(imgSizeB))
	db.DPrintf(db.ALWAYS, "Time %v resize: %v", t.inputs[i], time.Since(dr))

	// Only write back output if reading from SigmaOS
	if !localOS {
		dcw := time.Now()
		wrt, err := t.CreateWriter(output, 0777, sp.OWRITE)
		if err != nil {
			return proc.NewStatusErr(fmt.Sprintf("Open output failed %v", output), err)
		}
		//	pwrt := perf.NewPerfWriter(wrt, t.p)
		db.DPrintf(db.ALWAYS, "Time %v create writer: %v", t.inputs[i], time.Since(dcw))
		dw := time.Now()
		defer func() {
			wrt.Close()
			db.DPrintf(db.ALWAYS, "Time %v write/encode: %v", t.inputs[i], time.Since(dw))
			dc = time.Now()
		}()

		jpeg.Encode(wrt, img1, nil)
	}
	return proc.NewStatus(proc.StatusOK)
}
