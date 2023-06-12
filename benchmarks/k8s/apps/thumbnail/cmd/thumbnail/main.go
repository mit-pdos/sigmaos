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

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %v FILE_PATH\nArgs passed: %v", os.Args[0], os.Args)
	}
	t, err := MakeTrans(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	start := time.Now()
	t.Work()
	log.Printf("Time %v e2e resize: %v", os.Args, time.Since(start))
}

type Trans struct {
	input  string
	output string
}

func MakeTrans(args []string) (*Trans, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeReader: too few arguments")
	}
	log.Printf("MakeTrans: %v", args)
	t := &Trans{}
	t.input = args[1]
	t.output = args[1] + "-thumbnail-" + strconv.Itoa(rand.Int())
	return t, nil
}

func (t *Trans) Work() {
	do := time.Now()
	fs := corfs.InitFilesystem(corfs.S3)
	rdr, err := fs.OpenReader(t.input, 0)
	log.Printf("Time %v open: %v", t.input, time.Since(do))
	var dc time.Time
	defer func() {
		rdr.Close()
		log.Printf("Time %v close reader: %v", t.input, time.Since(dc))
	}()

	ds := time.Now()
	img, err := jpeg.Decode(rdr)
	log.Printf("Time %v read/decode: %v", t.input, time.Since(ds))
	dr := time.Now()
	img1 := resize.Resize(160, 0, img, resize.Lanczos3)
	log.Printf("Time %v resize: %v", t.input, time.Since(dr))

	dcw := time.Now()
	wrt, err := fs.OpenWriter(t.output)
	if err != nil {
		log.Fatalf("Open %v error: %v", t.output, err)
	}
	log.Printf("Time %v create writer: %v", t.input, time.Since(dcw))
	dw := time.Now()
	defer func() {
		wrt.Close()
		log.Printf("Time %v write/encode: %v", t.input, time.Since(dw))
		dc = time.Now()
	}()

	jpeg.Encode(wrt, img1, nil)
}
