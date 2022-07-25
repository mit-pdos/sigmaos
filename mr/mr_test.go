package mr_test

import (
	"bytes"
	"flag"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/mr"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procdclnt"
	rd "ulambda/rand"
	"ulambda/test"
)

const (
	OUTPUT = "par-mr.out"
	NCOORD = 5

	// time interval (ms) for when a failure might happen. If too
	// frequent and they don't finish ever. XXX determine
	// dynamically
	CRASHTASK  = 3000
	CRASHCOORD = 6000
	CRASHSRV   = 1000000
)

var app string // yaml app file
var job *mr.Job

func init() {
	flag.StringVar(&app, "app", "mr-wc.yml", "application")
}

func TestHash(t *testing.T) {
	assert.Equal(t, 0, mr.Khash("LEAGUE")%8)
	assert.Equal(t, 0, mr.Khash("Abbots")%8)
	assert.Equal(t, 0, mr.Khash("yes")%8)
	assert.Equal(t, 7, mr.Khash("absently")%8)
}

func TestSplits(t *testing.T) {
	ts := test.MakeTstateAll(t)
	job = mr.ReadJobConfig(app)
	bins, err := mr.MkBins(ts.FsLib, job.Input, np.Tlength(job.Binsz))
	assert.Nil(t, err)
	sum := np.Tlength(0)
	for _, b := range bins {
		n := np.Tlength(0)
		for _, s := range b {
			n += s.Length
		}
		sum += n
	}
	db.DPrintf(db.ALWAYS, "len %d %v sum %v\n", len(bins), bins, humanize.Bytes(uint64(sum)))
	assert.NotEqual(t, 0, len(bins))
	ts.Shutdown()
}

func TestSeqMRGrep(t *testing.T) {
	ts := test.MakeTstateAll(t)
	job = mr.ReadJobConfig(app)

	p := proc.MakeProc("user/seqgrep", []string{job.Input})
	err := ts.Spawn(p)
	assert.Nil(t, err)
	status, err := ts.WaitExit(p.Pid)
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())
	// assert.Equal(t, 795, n)
	ts.Shutdown()
}

type Tstate struct {
	job string
	*test.Tstate
	nreducetask int
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.Tstate = test.MakeTstateAll(t)
	job = mr.ReadJobConfig(app)
	ts.nreducetask = job.Nreduce
	ts.job = rd.String(4)

	// If we don't remove all temp files (and there are many left over from
	// previous runs of the tests), ux may be very slow and cause the test to
	// hang during intialization. Using RmDir on ux is slow too, so just do this
	// directly through the os for now.
	os.RemoveAll(path.Join(np.UXROOT, "mr"))

	mr.InitCoordFS(ts.FsLib, ts.job, ts.nreducetask)

	os.Remove(OUTPUT)

	return ts
}

func (ts *Tstate) compare() {
	cmd := exec.Command("sort", "seq-mr.out")
	var out1 bytes.Buffer
	cmd.Stdout = &out1
	err := cmd.Run()
	if err != nil {
		db.DPrintf(db.ALWAYS, "cmd err %v\n", err)
	}
	cmd = exec.Command("sort", OUTPUT)
	var out2 bytes.Buffer
	cmd.Stdout = &out2
	err = cmd.Run()
	if err != nil {
		db.DPrintf(db.ALWAYS, "cmd err %v\n", err)
	}
	b1 := out1.Bytes()
	b2 := out2.Bytes()
	assert.Equal(ts.T, len(b1), len(b2), "Output files have different length")
	assert.Equal(ts.T, b1, b2, "Output files have different contents")
}

func (ts *Tstate) checkJob() {
	err := mr.MergeReducerOutput(ts.FsLib, ts.job, OUTPUT, ts.nreducetask)
	assert.Nil(ts.T, err, "Merge output files: %v", err)
	if app == "wc" {
		ts.compare()
	}
}

// Sleep for a random time, then crash a server.  Crash a server of a
// certain type, then crash a server of that type.
func (ts *Tstate) crashServer(srv string, randMax int, l *sync.Mutex, crashchan chan bool) {
	r := rand.Intn(randMax)
	time.Sleep(time.Duration(r) * time.Microsecond)
	db.DPrintf(db.ALWAYS, "Crashing a %v after %v", srv, time.Duration(r)*time.Microsecond)
	// Make sure not too many crashes happen at once by taking a lock (we always
	// want >= 1 server to be up).
	l.Lock()
	switch srv {
	case np.PROCD:
		err := ts.BootProcd()
		assert.Nil(ts.T, err, "Spawn procd %v", err)
	case np.UX:
		err := ts.BootFsUxd()
		assert.Nil(ts.T, err, "Spawn uxd %v", err)
	default:
		assert.False(ts.T, true, "%v: Unrecognized service type", proc.GetProgram())
	}
	db.DPrintf(db.ALWAYS, "Kill one %v", srv)
	err := ts.KillOne(srv)
	assert.Nil(ts.T, err, "Kill procd %v", err)
	l.Unlock()
	crashchan <- true
}

func runN(t *testing.T, crashtask, crashcoord, crashprocd, crashux int, monitor bool) {
	ts := makeTstate(t)

	pdc := procdclnt.MakeProcdClnt(ts.FsLib, ts.RealmId())
	if monitor {
		pdc.MonitorProcds()
		defer pdc.Done()
	}

	nmap, err := mr.PrepareJob(ts.FsLib, ts.job, job)
	assert.Nil(ts.T, err)
	assert.NotEqual(ts.T, 0, nmap)

	cm := mr.StartMRJob(ts.FsLib, ts.ProcClnt, ts.job, job, mr.NCOORD, nmap, crashtask, crashcoord)

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

	err = mr.PrintMRStats(ts.FsLib, ts.job)
	assert.Nil(ts.T, err, "Error print MR stats: %v", err)

	ts.Shutdown()
}

func TestMRJOB(t *testing.T) {
	runN(t, 0, 0, 0, 0, true)
}

func TestCrashTaskOnly(t *testing.T) {
	runN(t, CRASHTASK, 0, 0, 0, false)
}

func TestCrashCoordOnly(t *testing.T) {
	runN(t, 0, CRASHCOORD, 0, 0, false)
}

func TestCrashTaskAndCoord(t *testing.T) {
	runN(t, CRASHTASK, CRASHCOORD, 0, 0, false)
}

func TestCrashProcd1(t *testing.T) {
	runN(t, 0, 0, 1, 0, false)
}

func TestCrashProcd2(t *testing.T) {
	N := 2
	runN(t, 0, 0, N, 0, false)
}

func TestCrashProcdN(t *testing.T) {
	N := 5
	runN(t, 0, 0, N, 0, false)
}

func TestCrashUx1(t *testing.T) {
	N := 1
	runN(t, 0, 0, 0, N, false)
}

func TestCrashUx2(t *testing.T) {
	N := 2
	runN(t, 0, 0, 0, N, false)
}

func TestCrashUx5(t *testing.T) {
	N := 5
	runN(t, 0, 0, 0, N, false)
}

func TestCrashProcdUx5(t *testing.T) {
	N := 5
	runN(t, 0, 0, N, N, false)
}
