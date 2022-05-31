package mr_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/groupmgr"
	"ulambda/mr"
	np "ulambda/ninep"
	"ulambda/proc"
	rd "ulambda/rand"
	"ulambda/realm"
	"ulambda/test"
)

const (
	OUTPUT = "par-mr.out"
	NCOORD = 5

	// time interval (ms) for when a failure might happen. If too
	// frequent and they don't finish ever. XXX determine
	// dynamically
	CRASHTASK  = 3000
	CRASHCOORD = 500
	CRASHSRV   = 1000000
)

var realmaddr string // Use this realm to run MR (instead of starting a new one)
var app string       // yaml app file
var job *Job

type Job struct {
	App     string `yalm:"app"`
	Nreduce int    `yalm:"nreduce"`
	Binsz   int    `yalm:"binsz"`
	Input   string `yalm:"input"`
	Linesz  int    `yalm:"linesz"`
	Ncore   int    `yalm:"ncore"`
}

func init() {
	flag.StringVar(&realmaddr, "realm", "", "realm id")
	flag.StringVar(&app, "app", "mr-wc.yml", "application")
}

func readConfig() *Job {
	job := &Job{}
	file, err := os.Open(app)
	if err != nil {
		log.Fatalf("ReadConfig err %v\n", err)
	}
	defer file.Close()
	d := yaml.NewDecoder(file)
	if err := d.Decode(&job); err != nil {
		log.Fatalf("Yalm decode %s err %v\n", app, err)
	}
	return job
}

func TestHash(t *testing.T) {
	assert.Equal(t, 0, mr.Khash("LEAGUE")%8)
	assert.Equal(t, 0, mr.Khash("Abbots")%8)
	assert.Equal(t, 0, mr.Khash("yes")%8)
	assert.Equal(t, 7, mr.Khash("absently")%8)
}

func TestSplits(t *testing.T) {
	ts := test.MakeTstateAll(t)
	job = readConfig()
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
	log.Printf("len %d %v sum %v\n", len(bins), bins, humanize.Bytes(uint64(sum)))
	assert.NotEqual(t, 0, len(bins))
	ts.Shutdown()
}

type Tstate struct {
	job string
	*test.Tstate
	nreducetask int
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	if realmaddr == "" {
		ts.Tstate = test.MakeTstateAll(t)
	} else {
		rconfig := realm.GetRealmConfig(fslib.MakeFsLib("test"), realmaddr)
		ts.Tstate = test.MakeTstateClnt(t, rconfig.NamedAddrs[0])
	}
	job = readConfig()
	ts.nreducetask = job.Nreduce
	ts.job = rd.String(4)

	// If we don't remove all temp files (and there are many left over from
	// previous runs of the tests), ux may be very slow and cause the test to
	// hang during intialization. Using RmDir on ux is slow too, so just do this
	// directly through the os for now.
	os.RemoveAll("/tmp/ulambda/mr")

	mr.InitCoordFS(ts.System.FsLib, ts.job, ts.nreducetask)

	os.Remove(OUTPUT)

	return ts
}

func (ts *Tstate) compare() {
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
	assert.Equal(ts.T, len(b1), len(b2), "Output files have different length")
	assert.Equal(ts.T, b1, b2, "Output files have different contents")
}

// Put names of input files in name/mr/m
func (ts *Tstate) prepareJob() int {
	bins, err := mr.MkBins(ts.FsLib, job.Input, np.Tlength(job.Binsz))
	assert.Nil(ts.T, err)
	assert.NotEqual(ts.T, 0, len(bins))
	for i, b := range bins {
		n := mr.MapTask(ts.job) + "/" + mr.BinName(i)
		_, err = ts.PutFile(n, 0777, np.OWRITE, []byte{})
		assert.Nil(ts.T, err, nil)
		for _, s := range b {
			err := ts.AppendFileJson(n, s)
			assert.Nil(ts.T, err, nil)
		}
	}
	return len(bins)
}

func (ts *Tstate) checkJob() {
	file, err := os.OpenFile(OUTPUT, os.O_WRONLY|os.O_CREATE, 0644)
	assert.Nil(ts.T, err, "Open output file: %v", err)
	defer file.Close()

	// XXX run as a proc?
	for i := 0; i < ts.nreducetask; i++ {
		r := strconv.Itoa(i)
		data, err := ts.GetFile(mr.ReduceOut(ts.job) + r)
		assert.Nil(ts.T, err, "GetFile %v err %v", r, err)
		_, err = file.Write(data)
		assert.Nil(ts.T, err, "Write err %v", err)
	}

	if app == "wc" {
		ts.compare()
	}
}

func (ts *Tstate) stats() {
	rdr, err := ts.OpenReader(mr.MRstats(ts.job))
	assert.Nil(ts.T, err)
	dec := json.NewDecoder(rdr)
	fmt.Println("=== STATS:")
	totIn := np.Tlength(0)
	totOut := np.Tlength(0)
	totWTmp := np.Tlength(0)
	totRTmp := np.Tlength(0)
	for {
		var r mr.Result
		if err := dec.Decode(&r); err == io.EOF {
			break
		}
		assert.Nil(ts.T, err)
		fmt.Printf("%s: in %s out %s %vms (%s)\n", r.Task, humanize.Bytes(uint64(r.In)), humanize.Bytes(uint64(r.Out)), r.Ms, test.Tput(r.In, r.Ms))
		if r.IsM {
			totIn += r.In
			totWTmp += r.Out
		} else {
			totOut += r.Out
			totRTmp += r.In
		}
	}
	fmt.Printf("=== totIn %s (%d) totOut %s tmpOut %s tmpIn %s\n",
		humanize.Bytes(uint64(totIn)), totIn,
		humanize.Bytes(uint64(totOut)),
		humanize.Bytes(uint64(totWTmp)),
		humanize.Bytes(uint64(totRTmp)),
	)
}

// Sleep for a random time, then crash a server.  Crash a server of a
// certain type, then crash a server of that type.
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
		assert.Nil(ts.T, err, "Spawn procd %v", err)
	case np.UX:
		err := ts.BootFsUxd()
		assert.Nil(ts.T, err, "Spawn uxd %v", err)
	default:
		assert.False(ts.T, true, "%v: Unrecognized service type", proc.GetProgram())
	}
	log.Printf("Kill one %v", srv)
	err := ts.KillOne(srv)
	assert.Nil(ts.T, err, "Kill procd %v", err)
	l.Unlock()
	crashchan <- true
}

func runN(t *testing.T, crashtask, crashcoord, crashprocd, crashux int) {
	ts := makeTstate(t)

	nmap := ts.prepareJob()

	cm := groupmgr.Start(ts.FsLib, ts.ProcClnt, mr.NCOORD, "bin/user/mr-coord", []string{ts.job, strconv.Itoa(nmap), strconv.Itoa(job.Nreduce), "bin/user/mr-m-" + job.App, "bin/user/mr-r-" + job.App, strconv.Itoa(crashtask), strconv.Itoa(job.Ncore), strconv.Itoa(job.Linesz)}, mr.NCOORD, crashcoord, 0, 0)

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

	ts.stats()

	if realmaddr == "" {
		ts.Shutdown()
	}
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
