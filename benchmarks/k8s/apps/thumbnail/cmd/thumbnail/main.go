package main

import (
	"errors"
	"fmt"
	"image/jpeg"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/ArielSzekely/corral/export/pkg/corfs"
	"github.com/nfnt/resize"
)

const N_ITER = 1

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %v FILE_PATH\nArgs passed: %v", os.Args[0], os.Args)
	}
	t, err := MakeTrans(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	for i := 0; i < N_ITER; i++ {
		start := time.Now()
		t.Work(t.outputbase + strconv.Itoa(rand.Int()))
		log.Printf("Time %v e2e resize: %v", os.Args, time.Since(start))
	}
}

type Trans struct {
	input      string
	outputbase string
}

func MakeTrans(args []string) (*Trans, error) {
	if len(args) != 2 {
		return nil, errors.New("MakeReader: too few arguments")
	}
	log.Printf("MakeTrans: %v", args)
	t := &Trans{}
	t.input = args[1]
	rand.Seed(time.Now().UnixNano())
	t.outputbase = args[1] + "-thumbnail-"
	return t, nil
}

func (t *Trans) Work(output string) {
	log.Printf("Output %v", output)
	si := time.Now()
	fs := corfs.InitFilesystem(corfs.S3)
	log.Printf("Time %v init fs: %v", t.input, time.Since(si))
	do := time.Now()
	rdr, err := fs.OpenReader(t.input, 0)
	log.Printf("Time %v open: %v", t.input, time.Since(do))
	if err != nil {
		log.Fatalf("Error OpenReader: %v", err)
	}
	var dc time.Time
	defer func() {
		rdr.Close()
		log.Printf("Time %v close reader: %v", t.input, time.Since(dc))
	}()

	ds := time.Now()
	img, err := jpeg.Decode(rdr)
	if err != nil {
		log.Fatalf("Error decode jpeg: %v", err)
	}
	log.Printf("Time %v read/decode: %v", t.input, time.Since(ds))
	dr := time.Now()
	img1 := resize.Resize(160, 0, img, resize.Lanczos3)
	log.Printf("Time %v resize: %v", t.input, time.Since(dr))

	dcw := time.Now()
	wrt, err := fs.OpenWriter(output)
	if err != nil {
		log.Fatalf("Open %v error: %v", output, err)
	}
	log.Printf("Time %v create writer: %v", t.input, time.Since(dcw))
	dw := time.Now()
	defer func() {
		wrt.Close()
		log.Printf("Time %v write/encode: %v", t.input, time.Since(dw))
		dc = time.Now()
	}()

	jpeg.Encode(wrt, img1, nil)
	log.Printf("Success!")
}