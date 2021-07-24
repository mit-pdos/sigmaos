package perf

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"math/rand"
	"os"
	"os/user"
	"path"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"ulambda/fslib"
	np "ulambda/ninep"
)

const (
	MB     = 1000000
	N_RUNS = 1000
)

var bucket = "9ps3"
var key = "write-bandwidth-test"
var fname = "name/bigfile.txt"

type BandwidthTest struct {
	bytes  int
	memfs  bool
	client *s3.Client
	*fslib.FsLib
}

func MakeBandwidthTest(args []string) (*BandwidthTest, error) {
	if len(args) < 2 {
		return nil, errors.New("MakeBandwidthTest: too few arguments")
	}
	log.Printf("MakeBandwidthTest: %v\n", args)

	t := &BandwidthTest{}
	t.FsLib = fslib.MakeFsLib("write-bandwidth-test")

	bytes, err := strconv.Atoi(args[0])
	t.bytes = bytes
	if err != nil {
		log.Fatalf("Invalid num MB: %v, %v\n", args[0], err)
	}

	if args[1] == "memfs" {
		t.memfs = true
	} else if args[1] == "s3" {
		t.memfs = false
	} else {
		log.Fatalf("Unknown test type: %v", args[1])
	}

	// Set up s3 client
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile("default"))
	if err != nil {
		log.Fatalf("Failed to load SDK configuration %v", err)
	}

	t.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return t, nil
}

func (t *BandwidthTest) FillBuf(buf []byte) {
	rand.Read(buf)
}

func (t *BandwidthTest) S3Write(buf []byte) time.Duration {
	inputs := []*s3.PutObjectInput{}
	for i := 0; i < N_RUNS; i++ {
		r1 := bytes.NewReader(buf)
		input := &s3.PutObjectInput{
			Bucket: &bucket,
			Key:    &key,
			Body:   r1,
		}
		inputs = append(inputs, input)
	}
	start := time.Now()
	for i := 0; i < N_RUNS; i++ {
		_, err := t.client.PutObject(context.TODO(), inputs[i])
		if err != nil {
			log.Printf("Error putting s3 object: %v", err)
		}
	}
	end := time.Now()
	elapsed := end.Sub(start)
	return elapsed
}

func (t *BandwidthTest) S3Read(buf []byte) time.Duration {
	// setup
	region := "bytes=0-" + strconv.Itoa(len(buf))
	input := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Range:  &region,
	}
	buf2 := make([]byte, len(buf))

	var n int
	// timing
	start := time.Now()
	for i := 0; i < N_RUNS; i++ {
		result, err := t.client.GetObject(context.TODO(), input)
		if err != nil {
			log.Fatalf("Error getting s3 object: %v", err)
		}
		n = 0
		// Have to include this in timing since GetObjectOutput seems to read in
		// chunks.
		for {
			n1, err := result.Body.Read(buf2[n:])
			n += n1
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("Error reading s3 object result: %v", err)
			}
		}
	}
	end := time.Now()
	elapsed := end.Sub(start)

	if n != len(buf) {
		log.Fatalf("Length of s3 read buffer didn't match: %v, %v", n, len(buf))
	}
	for i := range buf {
		if buf2[i] != buf[i] {
			log.Fatalf("S3 Read buf didn't match written buf at index %v", i)
		}
	}
	return elapsed
}

func (t *BandwidthTest) MemfsWrite(buf []byte) time.Duration {
	// setup
	err := t.MakeFile(fname, 0777, np.OWRITE, []byte{})
	if err != nil && err.Error() != "Name exists" {
		log.Fatalf("Error creating file: %v", err)
	}

	// timing
	start := time.Now()
	for i := 0; i < N_RUNS; i++ {
		err = t.WriteFile(fname, buf)
		if err != nil {
			log.Fatalf("Error writefile memfs: %v", err)
		}
	}
	end := time.Now()
	elapsed := end.Sub(start)

	return elapsed
}

func (t *BandwidthTest) MemfsRead(buf []byte) time.Duration {
	// timing
	var buf2 []byte
	var err error
	start := time.Now()
	for i := 0; i < N_RUNS; i++ {
		buf2, err = t.ReadFile(fname)
		if err != nil {
			log.Fatalf("ReadFile er not nil: %v", err)
		}
	}
	end := time.Now()
	elapsed := end.Sub(start)

	for i := range buf {
		if buf2[i] != buf[i] {
			log.Fatalf("Memfs Read buf didn't match written buf at index %v", i)
		}
	}

	// cleanup
	err = t.Remove(fname)
	if err != nil {
		log.Printf("Error removing file: %v", err)
	}
	return elapsed
}

func (t *BandwidthTest) Work() {
	buf := make([]byte, t.bytes)
	t.FillBuf(buf)

	// ===== Profiling code =====
	usr, err := user.Current()
	if err != nil {
		log.Fatalf("Error getting current user: %v", err)
	}
	f, err := os.Create(path.Join(usr.HomeDir, "client.out"))
	if err != nil {
		log.Printf("Couldn't make profile file")
	}
	defer f.Close()
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatalf("Couldn't start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()
	// ===== Profiling code =====

	var elapsedWrite time.Duration
	var elapsedRead time.Duration
	if t.memfs {
		elapsedWrite = t.MemfsWrite(buf)
		elapsedRead = t.MemfsRead(buf)
	} else {
		elapsedWrite = t.S3Write(buf)
		elapsedRead = t.S3Read(buf)
	}
	log.Printf("Bytes: %v, Runs: %v", t.bytes, N_RUNS)
	log.Printf("Avg Write Time: %f (usec)", float64(elapsedWrite.Microseconds())/float64(N_RUNS))
	log.Printf("Avg Write Throughput: %f (Mb/sec)", 8.0*float64(t.bytes)/float64(MB)/(elapsedWrite.Seconds()/float64(N_RUNS)))
	log.Printf("Avg Read Time: %f (usec)", float64(elapsedRead.Microseconds())/float64(N_RUNS))
	log.Printf("Avg Read Throughput: %f (Mb/sec)", 8.0*float64(t.bytes)/float64(MB)/(elapsedRead.Seconds()/float64(N_RUNS)))
}
