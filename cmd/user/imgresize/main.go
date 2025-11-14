package main

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nfnt/resize"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/proc"
	s3clnt "sigmaos/proxy/s3/clnt"
	"sigmaos/sigmaclnt"
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

	ip, err := NewImgProcess(pe, os.Args, p)
	if err != nil {
		db.DFatalf("Error %v", err)
	}

	rand.Seed(time.Now().UnixNano())

	var s *proc.Status
	for i := 0; i < len(ip.inputs); i++ {
		start := time.Now()
		output := ip.output
		// Create a new file name for iterations > 0
		output += strconv.Itoa(rand.Int())
		s = ip.Work(i, output)
		db.DPrintf(db.ALWAYS, "Time %v e2e resize[%v]: %v", os.Args, i, time.Since(start))
	}
	ip.ClntExit(s)
}

type ImgProcess struct {
	*sigmaclnt.SigmaClnt
	inputs    []string
	output    string
	ctx       fs.CtxI
	nrounds   int
	p         *perf.Perf
	useS3Clnt bool
	s3Clnt    *s3clnt.S3Clnt
}

func NewImgProcess(pe *proc.ProcEnv, args []string, p *perf.Perf) (*ImgProcess, error) {
	if len(args) != 5 {
		return nil, fmt.Errorf("NewImgProcess: wrong number of arguments: %v", args)
	}
	ip := &ImgProcess{
		p: p,
	}
	db.DPrintf(db.ALWAYS, "E2e spawn time since spawn until main: %v", time.Since(pe.GetSpawnTime()))
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return nil, err
	}
	ip.SigmaClnt = sc
	ip.inputs = strings.Split(args[1], ",")
	db.DPrintf(db.ALWAYS, "Args {%v} inputs {%v} fail {%v}", args[1], ip.inputs, proc.GetSigmaFail())
	ip.output = ip.inputs[0] + "-thumbnail"
	ip.nrounds, err = strconv.Atoi(args[3])
	if err != nil {
		db.DFatalf("Err convert nrounds: %v", err)
	}
	ip.Started()
	crash.FailersDefault(sc.FsLib, []crash.Tselector{crash.IMGRESIZE_CRASH})
	useS3Clnt, err := strconv.ParseBool(args[4])
	if err != nil {
		db.DFatalf("Err parse useS3Clnt: %v", err)
	}
	ip.useS3Clnt = useS3Clnt
	if useS3Clnt {
		s3Clnt, err := s3clnt.NewS3Clnt(ip.FsLib, filepath.Join(sp.S3, pe.GetKernelID()))
		if err != nil {
			db.DFatalf("Err newS3Clnt: %v", err)
		}
		ip.s3Clnt = s3Clnt
	}
	return ip, nil
}

func (ip *ImgProcess) Work(i int, output string) *proc.Status {
	db.DPrintf(db.ALWAYS, "Resize (%v/%v) %v", i, len(ip.inputs), ip.inputs[i])
	do := time.Now()
	var rdr io.ReadCloser
	var err error
	if ip.useS3Clnt {
		pn := strings.Split(ip.inputs[i], "/")
		bucket := pn[0]
		key := pn[1]
		b, err := ip.s3Clnt.GetObject(bucket, key)
		if err != nil {
			return proc.NewStatusErr(fmt.Sprintf("Err GetObject bucket:%v key:%v", bucket, key), err)
		}
		rdr = io.NopCloser(bytes.NewReader(b))
	} else {
		rdr, err = ip.OpenReader(ip.inputs[i])
		if err != nil {
			return proc.NewStatusErr(fmt.Sprintf("File %v not found kid %v", ip.inputs[i], ip.ProcEnv().GetKernelID()), err)
		}
	}
	//	prdr := perf.NewPerfReader(rdr, ip.p)
	db.DPrintf(db.ALWAYS, "Time %v open: %v", ip.inputs[i], time.Since(do))
	var dc time.Time
	defer func() {
		rdr.Close()
		db.DPrintf(db.ALWAYS, "Time %v close reader: %v", ip.inputs[i], time.Since(dc))
	}()

	ds := time.Now()
	img, err := jpeg.Decode(rdr)
	if err != nil {
		return proc.NewStatusErr("Decode", err)
	}
	// img size in bytes:
	bounds := img.Bounds()
	var imgSizeB uint64 = 16 * uint64(bounds.Max.X-bounds.Min.X) * uint64(bounds.Max.Y-bounds.Min.Y)
	db.DPrintf(db.ALWAYS, "Time %v read/decode: %v", ip.inputs[i], time.Since(ds))
	dr := time.Now()
	for i := 0; i < ip.nrounds-1; i++ {
		resize.Resize(IMG_DIM, IMG_DIM, img, resize.Lanczos3)
		ip.p.TptTick(float64(imgSizeB))
	}
	img1 := resize.Resize(IMG_DIM, IMG_DIM, img, resize.Lanczos3)
	ip.p.TptTick(float64(imgSizeB))
	db.DPrintf(db.ALWAYS, "Time %v resize: %v", ip.inputs[i], time.Since(dr))

	dcw := time.Now()
	wrt, err := ip.CreateWriter(output, 0777, sp.OWRITE)
	if err != nil {
		return proc.NewStatusErr(fmt.Sprintf("Open output failed %v", output), err)
	}
	//	pwrt := perf.NewPerfWriter(wrt, ip.p)
	db.DPrintf(db.ALWAYS, "Time %v create writer: %v", ip.inputs[i], time.Since(dcw))
	dw := time.Now()
	defer func() {
		wrt.Close()
		db.DPrintf(db.ALWAYS, "Time %v write/encode: %v", ip.inputs[i], time.Since(dw))
		dc = time.Now()
	}()

	jpeg.Encode(wrt, img1, nil)
	return proc.NewStatus(proc.StatusOK)
}
