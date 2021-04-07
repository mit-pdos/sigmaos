package perf

import (
	"errors"
	"log"
	"strconv"
	"time"

	// db "ulambda/debug"
	"ulambda/fslib"
)

const (
	MB = 1000000
)

type MemfsBandwidthTest struct {
	mb int
	*fslib.FsLib
}

func MakeMemfsBandwidthTest(args []string) (*MemfsBandwidthTest, error) {
	if len(args) < 1 {
		return nil, errors.New("MakeMemfsBandwidthTest: too few arguments")
	}
	log.Printf("MakeMemfsBandwidthTest: %v\n", args)

	t := &MemfsBandwidthTest{}
	t.FsLib = fslib.MakeFsLib("memfs-bandwidth-test")

	mb, err := strconv.Atoi(args[0])
	t.mb = mb
	if err != nil {
		log.Fatalf("Invalid num MB: %v, %v\n", args[0], err)
	}

	return t, nil
}

func (t *MemfsBandwidthTest) FillBuf(buf []byte) {
	for i := range buf {
		buf[i] = byte(i)
	}
}

func (t *MemfsBandwidthTest) Work() {
	buf := make([]byte, t.mb*MB)
	t.FillBuf(buf)
	fname := "name/fs/bigfile.txt"
	err := t.MakeFile(fname, []byte{})
	if err != nil {
		log.Printf("Error creating file: %v", err)
	}
	start := time.Now()
	err = t.WriteFile(fname, buf)
	end := time.Now()
	elapsed := end.Sub(start)
	err = t.Remove(fname)
	if err != nil {
		log.Printf("Error removing file: %v", err)
	}
	log.Printf("Time: %v (usec)", elapsed.Microseconds())
	log.Printf("Bytes: %v", t.mb*MB)
	log.Printf("Throughput: %f (MB/sec)", float64(t.mb)/float64(elapsed.Seconds()))
}
