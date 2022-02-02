package mr_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	"ulambda/fslib"
	"ulambda/groupmgr"
	"ulambda/kernel"
	"ulambda/mr"
	np "ulambda/ninep"
)

const (
	OUTPUT = "par-mr.out"
	NCOORD = 5

	// time interval (ms) for when a failure might happen. If too
	// frequent and they don't finish ever. XXX determine
	// dynamically
	CRASHTASK  = 10000
	CRASHCOORD = 20000
	CRASHSRV   = 700000
)

func Compare(fsl *fslib.FsLib) {
	cmd := exec.Command("sort", "seq-mr.out")
	var out1 bytes.Buffer
	cmd.Stdout = &out1
	err := cmd.Run()
	if err != nil {
		log.Printf("cmd err %v\n", err)
	}
	cmd = exec.Command("sort", OUTPUT)
	var out2 bytes.Buffer
	cmd.Stdout = &out2
	err = cmd.Run()
	if err != nil {
		log.Printf("cmd err %v\n", err)
	}
	b1 := out1.Bytes()
	b2 := out2.Bytes()
	if len(b1) != len(b2) {
		log.Fatalf("Output files have different length\n")
	}
	for i, v := range b1 {
		if v != b2[i] {
			log.Fatalf("Buf %v diff %v %v\n", i, v, b2[i])
			break
		}
	}
}

type Tstate struct {
	*kernel.System
	t           *testing.T
	nreducetask int
}

func makeTstate(t *testing.T, nreducetask int) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.System = kernel.MakeSystemAll("mr-wc-test", "..")
	ts.nreducetask = nreducetask

	mr.InitCoordFS(ts.System.FsLib, nreducetask)

	os.Remove(OUTPUT)

	return ts
}

func (ts *Tstate) prepareJob() {
	// Put names of input files in name/mr/m
	files, err := ioutil.ReadDir("../input/")
	if err != nil {
		log.Fatalf("Readdir %v\n", err)
	}
	for _, f := range files {
		// remove mapper output directory from previous run
		ts.RmDir("name/ux/~ip/m-" + f.Name())
		n := mr.MDIR + "/" + f.Name()
		if _, err := ts.PutFile(n, []byte(n), 0777, np.OWRITE); err != nil {
			log.Fatalf("PutFile %v err %v\n", n, err)
		}
	}
}

func (ts *Tstate) checkJob() {
	file, err := os.OpenFile(OUTPUT, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("Couldn't open output file\n")
	}
	defer file.Close()

	// XXX run as a proc?
	for i := 0; i < ts.nreducetask; i++ {
		r := strconv.Itoa(i)
		data, err := ts.ReadFile(mr.ROUT + r)
		if err != nil {
			log.Fatalf("ReadFile %v err %v\n", r, err)
		}
		_, err = file.Write(data)
		if err != nil {
			log.Fatalf("Write err %v\n", err)
		}
	}

	Compare(ts.FsLib)
}

// Crash a server, then start a new one of the same type.
func (ts *Tstate) crashServer(srv string, randMax int, l *sync.Mutex, crashchan chan bool) {
	r := rand.Intn(randMax)
	time.Sleep(time.Duration(r) * time.Microsecond)
	// Make sure not too many crashes happen at once by taking a lock (we always
	// want >= 1 server to be up).
	l.Lock()
	err := ts.KillOne(srv)
	if err != nil {
		log.Fatalf("Error non-nil kill procd: %v", err)
	}
	err = ts.BootProcd()
	if err != nil {
		log.Fatalf("Error spawn procd")
	}
	l.Unlock()
	crashchan <- true
}

func runN(t *testing.T, crashtask, crashcoord, crashprocd, crashux int) {
	const NReduce = 2
	ts := makeTstate(t, NReduce)

	if crashprocd > 0 {
		ts.BootProcd()
	}

	ts.prepareJob()

	cm := groupmgr.Start(ts.FsLib, ts.ProcClnt, mr.NCOORD, "bin/user/mr-coord", []string{strconv.Itoa(NReduce), "bin/user/mr-m-wc", "bin/user/mr-r-wc", strconv.Itoa(crashtask)}, crashcoord)

	crashchan := make(chan bool)
	if crashprocd > 0 || crashux > 0 {
		l := &sync.Mutex{}
		for i := 0; i < crashprocd; i++ {
			// Sleep for a random time, then crash a server.
			go ts.crashServer(np.PROCD, (i+1)*CRASHSRV, l, crashchan)
		}
	}

	cm.Wait()

	if crashprocd > 0 || crashux > 0 {
		for i := 0; i < crashprocd; i++ {
			<-crashchan
		}
	}

	ts.checkJob()

	ts.Shutdown()
}

func TestOne(t *testing.T) {
	runN(t, 0, 0, 0, 0)
}

func TestCrashTaskOnly(t *testing.T) {
	runN(t, CRASHTASK, 0, 0, 0)
}

func TestCrashCoordOnly(t *testing.T) {
	runN(t, 0, CRASHCOORD, 0, 0)
}

func TestCrashTaskAndCoord(t *testing.T) {
	runN(t, CRASHTASK, CRASHCOORD, 0, 0)
}

func TestCrashProcd1(t *testing.T) {
	runN(t, 0, 0, 1, 0)
}

func TestCrashProcd2(t *testing.T) {
	N := 2
	runN(t, 0, 0, N, 0)
}

func TestCrashProcdN(t *testing.T) {
	N := 5
	runN(t, 0, 0, N, 0)
}

func TestCrashUx1(t *testing.T) {
	N := 1
	runN(t, 0, 0, 0, N)
}
