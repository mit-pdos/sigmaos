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

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/groupmgr"
	"ulambda/mr"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/test"
)

const (
	OUTPUT = "par-mr.out"
	NCOORD = 5

	// time interval (ms) for when a failure might happen. If too
	// frequent and they don't finish ever. XXX determine
	// dynamically
	CRASHTASK  = 2000
	CRASHCOORD = 5000
	CRASHSRV   = 1000000
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
		db.DFatalf("Output files have different length\n")
	}
	for i, v := range b1 {
		if v != b2[i] {
			db.DFatalf("Buf %v diff %v %v\n", i, v, b2[i])
			break
		}
	}
}

type Tstate struct {
	*test.Tstate
	nreducetask int
}

func makeTstate(t *testing.T, nreducetask int) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	ts.nreducetask = nreducetask

	mr.InitCoordFS(ts.System.FsLib, nreducetask)

	os.Remove(OUTPUT)

	return ts
}

// Put names of input files in name/mr/m
func (ts *Tstate) prepareJob() int {
	files, err := ioutil.ReadDir("../input/")
	if err != nil {
		db.DFatalf("Readdir %v\n", err)
	}
	for _, f := range files {
		// remove mapper output directory from previous run
		ts.RmDir(mr.Moutdir(f.Name()))
		n := mr.MDIR + "/" + f.Name()
		if _, err := ts.PutFile(n, 0777, np.OWRITE, []byte(n)); err != nil {
			db.DFatalf("PutFile %v err %v\n", n, err)
		}
		// break
	}
	return len(files)
}

func (ts *Tstate) checkJob() {
	file, err := os.OpenFile(OUTPUT, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		db.DFatalf("Couldn't open output file\n")
	}
	defer file.Close()

	// XXX run as a proc?
	for i := 0; i < ts.nreducetask; i++ {
		r := strconv.Itoa(i)
		data, err := ts.GetFile(mr.ROUT + r)
		if err != nil {
			db.DFatalf("ReadFile %v err %v\n", r, err)
		}
		_, err = file.Write(data)
		if err != nil {
			db.DFatalf("Write err %v\n", err)
		}
	}

	Compare(ts.FsLib)
}

// Crash a server of a certain type, then crash a server of that type.
func (ts *Tstate) crashServer(srv string, randMax int, l *sync.Mutex, crashchan chan bool) {
	r := rand.Intn(randMax)
	time.Sleep(time.Duration(r) * time.Microsecond)
	log.Printf("Crashing a %v after %v", srv, time.Duration(r)*time.Microsecond)
	// Make sure not too many crashes happen at once by taking a lock (we always
	// want >= 1 server to be up).
	l.Lock()
	switch srv {
	case np.PROCD:
		err := ts.BootProcd()
		if err != nil {
			db.DFatalf("Error spawn procd")
		}
	case np.UX:
		err := ts.BootFsUxd()
		if err != nil {
			db.DFatalf("Error spawn uxd")
		}
	default:
		db.DFatalf("%v: Unrecognized service type", proc.GetProgram())
	}
	log.Printf("Kill one %v", srv)
	err := ts.KillOne(srv)
	if err != nil {
		db.DFatalf("Error non-nil kill procd: %v", err)
	}
	l.Unlock()
	crashchan <- true
}

func runN(t *testing.T, crashtask, crashcoord, crashprocd, crashux int) {
	const NReduce = 2
	ts := makeTstate(t, NReduce)

	nmap := ts.prepareJob()

	cm := groupmgr.Start(ts.FsLib, ts.ProcClnt, mr.NCOORD, "bin/user/mr-coord", []string{strconv.Itoa(nmap), strconv.Itoa(NReduce), "bin/user/mr-m-wc", "bin/user/mr-r-wc", strconv.Itoa(crashtask)}, mr.NCOORD, crashcoord, 0, 0)

	crashchan := make(chan bool)
	l1 := &sync.Mutex{}
	for i := 0; i < crashprocd; i++ {
		// Sleep for a random time, then crash a server.
		go ts.crashServer(np.PROCD, (i+1)*CRASHSRV, l1, crashchan)
	}
	l2 := &sync.Mutex{}
	for i := 0; i < crashux; i++ {
		// Sleep for a random time, then crash a server.
		go ts.crashServer(np.UX, (i+1)*CRASHSRV, l2, crashchan)
	}

	cm.Wait()

	for i := 0; i < crashprocd+crashux; i++ {
		<-crashchan
	}

	ts.checkJob()

	ts.Shutdown()
}

func TestMR(t *testing.T) {
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

func TestCrashUx2(t *testing.T) {
	N := 2
	runN(t, 0, 0, 0, N)
}

func TestCrashUx5(t *testing.T) {
	N := 5
	runN(t, 0, 0, 0, N)
}

func TestCrashProcdUx5(t *testing.T) {
	N := 5
	runN(t, 0, 0, N, N)
}
