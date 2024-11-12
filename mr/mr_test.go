package mr_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/klauspost/readahead"
	"github.com/stretchr/testify/assert"

	"sigmaos/auth"
	"sigmaos/awriter"
	db "sigmaos/debug"
	"sigmaos/mr"
	"sigmaos/perf"
	"sigmaos/proc"
	rd "sigmaos/rand"
	"sigmaos/scheddclnt"
	"sigmaos/seqwc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"

	// "sigmaos/stats"
	"sigmaos/grep"
	"sigmaos/test"
	"sigmaos/wc"
)

const (
	OUTPUT        = "/tmp/par-mr.out"
	MALICIOUS_APP = "mr-wc-restricted.yml"

	// time interval (ms) for when a failure might happen. If too
	// frequent and they don't finish ever. XXX determine
	// dynamically
	CRASHTASK  = 500
	CRASHCOORD = 1000
	CRASHSRV   = 10000
	MEM_REQ    = 1000
)

var app string // yaml app file
var nmap int
var job *mr.Job
var timeout time.Duration

func init() {
	flag.StringVar(&app, "app", "mr-wc.yml", "application")
	flag.IntVar(&nmap, "nmap", 1, "number of mapper threads")
	flag.DurationVar(&timeout, "mr-timeout", 0, "timeout")
}

func TestCompile(t *testing.T) {
}

func TestHash(t *testing.T) {
	assert.Equal(t, 0, mr.Khash([]byte("LEAGUE"))%8)
	assert.Equal(t, 0, mr.Khash([]byte("Abbots"))%8)
	assert.Equal(t, 0, mr.Khash([]byte("yes"))%8)
	assert.Equal(t, 7, mr.Khash([]byte("absently"))%8)
}

func TestLocalWc(t *testing.T) {
	const (
		LOCALINPUT = "/tmp/enwiki-1G"
		HOSTTMP    = "/tmp/sigmaos"
		F          = "gutenberg.txt"
		// INPUT   = "../input/" + F
		INPUT = LOCALINPUT
		OUT   = HOSTTMP + F + ".out"
	)

	file, err := os.Open(INPUT)
	assert.Nil(t, err)
	defer file.Close()
	r := bufio.NewReader(file)
	rdr, err := readahead.NewReaderSize(r, 4, sp.BUFSZ)
	assert.Nil(t, err, "Err reader: %v", err)
	start := time.Now()
	scanner := bufio.NewScanner(rdr)
	buf := make([]byte, 0, 2097152)
	scanner.Buffer(buf, cap(buf))
	data := make(seqwc.Tdata, 0)
	p, err := perf.NewPerf(proc.NewTestProcEnv(sp.ROOTREALM, nil, nil, sp.NO_IP, sp.NO_IP, "", false, false, false), perf.SEQWC)
	assert.Nil(t, err)
	sbc := mr.NewScanByteCounter(p)
	for scanner.Scan() {
		l := scanner.Text()
		if len(l) > 0 {
			seqwc.Wcline(0, l, data, sbc)
		}
	}
	err = scanner.Err()
	assert.Nil(t, err)
	db.DPrintf(db.ALWAYS, "seqwc %v %v", INPUT, time.Since(start).Milliseconds())
	file, err = os.Create(OUT)
	assert.Nil(t, err)
	defer file.Close()
	aw := awriter.NewWriterSize(file, 4, sp.BUFSZ)
	bw := bufio.NewWriterSize(aw, sp.BUFSZ)
	defer bw.Flush()
	defer aw.Close()
	for k, v := range data {
		b := fmt.Sprintf("%s\t%d\n", k, v)
		_, err := bw.Write([]byte(b))
		assert.Nil(t, err)
	}
	p.Done()
}

func TestSplits(t *testing.T) {
	const SPLITSZ = 10 * sp.MBYTE
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	job, err1 = mr.ReadJobConfig(app)
	assert.Nil(t, err1, "Error ReadJobConfig: %v", err1)
	bins, err := mr.NewBins(ts.FsLib, job.Input, sp.Tlength(job.Binsz), SPLITSZ)
	assert.Nil(t, err)
	sum := sp.Tlength(0)
	for _, b := range bins {
		n := sp.Tlength(0)
		for _, s := range b {
			n += s.Length
		}
		sum += n
	}
	db.DPrintf(db.ALWAYS, "len %d %v sum %v\n", len(bins), bins, humanize.Bytes(uint64(sum)))
	assert.NotEqual(t, 0, len(bins))
	ts.Shutdown()
}

func TestMapperReducer(t *testing.T) {
	t1, err := test.NewTstateAll(t)
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	ts := newTstate(t1, mr.MRDIRTOP, app) // or --app mr-wc-ux.yml or --app mr-ux-wiki1G.yml

	if job.Local != "" {
		err := ts.UploadDir(job.Local, job.Input)
		assert.Nil(t, err, "UploadDir %v %v err %v", job.Local, job.Input, err)
	}

	nmap, err := mr.PrepareJob(ts.FsLib, ts.tasks, ts.jobRoot, ts.job, job)
	assert.Nil(ts.T, err, "PrepareJob err %v: %v", job, err)
	assert.NotEqual(ts.T, 0, nmap)

	mapper := wc.Map
	reducer := wc.Reduce
	if job.App == "grep" {
		mapper = grep.Map
		reducer = grep.Reduce
	}

	p, err := perf.NewPerf(proc.NewTestProcEnv(sp.ROOTREALM, nil, nil, sp.NO_IP, sp.NO_IP, "", false, false, false), perf.MRMAPPER)
	assert.Nil(t, err)

	tns, err := ts.tasks.Mft.AcquireTasks()
	assert.Nil(t, err)

	start := time.Now()
	nin := sp.Tlength(0)
	nout := sp.Tlength(0)
	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	nmapper := len(tns)
	outBins := make([]mr.Bin, nmapper)
	db.DPrintf(db.TEST, "nmapper: %d %d", nmapper, job.Binsz)
	for i, task := range tns {
		input := ts.tasks.Mft.TaskPathName(task)
		bin, err := ts.GetFile(input)
		assert.Nil(t, err)
		start := time.Now()
		sc, err := sigmaclnt.NewSigmaClnt(pe)
		assert.Nil(t, err, "NewSC: %v", err)
		db.DPrintf(db.TEST, "NewSigmaClnt %v", time.Since(start))
		start = time.Now()
		m, err := mr.NewMapper(sc, mapper, reducer, ts.jobRoot, ts.job, p, job.Nreduce, job.Linesz, string(bin), job.Intermediate, true)
		assert.Nil(t, err, "NewMapper %v", err)
		db.DPrintf(db.TEST, "Newmapper %v", time.Since(start))
		start = time.Now()
		in, out, obin, err := m.DoMap()
		assert.Nil(t, err)
		outBins[i] = obin
		nin += in
		nout += out
		db.DPrintf(db.ALWAYS, "map %s: in %s out %s tot %s %vms (%s)\n", input, humanize.Bytes(uint64(in)), humanize.Bytes(uint64(out)), humanize.Bytes(uint64(in+out)), time.Since(start).Milliseconds(), test.TputStr(in+out, time.Since(start).Milliseconds()))
	}
	db.DPrintf(db.ALWAYS, "map %s total: in %s out %s tot %s %vms (%s)\n", job.Input, humanize.Bytes(uint64(nin)), humanize.Bytes(uint64(nout)), humanize.Bytes(uint64(nin+nout)), time.Since(start).Milliseconds(), test.TputStr(nin+nout, time.Since(start).Milliseconds()))

	tns, err = ts.tasks.Rft.AcquireTasks()
	assert.Nil(t, err)

	for i, task := range tns {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		sc, err := sigmaclnt.NewSigmaClnt(pe)
		assert.Nil(t, err)
		rt := &mr.TreduceTask{}
		err = ts.tasks.Rft.ReadTask(task, rt)
		assert.Nil(t, err)

		b := make(mr.Bin, nmapper)
		for j := 0; j < len(b); j++ {
			b[j] = outBins[j][i]
		}
		db.DPrintf(db.TEST, "reducer %d: %v", i, b)
		d, err := json.Marshal(b)
		assert.Nil(t, err)

		outlink := mr.ReduceOut(ts.jobRoot, ts.job) + rt.Task
		outTarget := mr.ReduceOutTarget(job.Output, ts.job) + rt.Task

		r, err := mr.NewReducer(sc, reducer, []string{string(d), outlink, outTarget, strconv.Itoa(nmap), "true"}, p)
		assert.Nil(t, err)
		status := r.DoReduce()
		assert.True(t, status.IsStatusOK(), "status %v", status)
		res, err := mr.NewResult(status.Data())
		assert.Nil(t, err)
		db.DPrintf(db.ALWAYS, "%s: in %v out %v tot %v %vms (%s)\n", res.Task, humanize.Bytes(uint64(res.In)), humanize.Bytes(uint64(res.Out)), test.Mbyte(res.In+res.Out), res.MsInner, test.TputStr(res.In+res.Out, res.MsInner))
	}

	if app == "mr-wc.yml" || app == "mr-ux-wc.yml" {
		ts.checkJob(app)
	}

	p.Done()
	ts.Shutdown()
}

func TestSeqGrep(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	job, err1 = mr.ReadJobConfig(app)
	assert.Nil(t, err1, "Error ReadJobConfig: %v", err1)

	p := proc.NewProc("seqgrep", []string{job.Input})
	err := ts.Spawn(p)
	assert.Nil(t, err)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())
	// assert.Equal(t, 795, n)
	ts.Shutdown()
}

func TestSeqWc(t *testing.T) {
	const OUT = "name/ux/~local/seqout.txt"
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	job, err1 = mr.ReadJobConfig(app)
	assert.Nil(t, err1, "Error ReadJobConfig: %v", err1)

	ts.Remove(OUT)

	p := proc.NewProc("seqwc", []string{job.Input, OUT})
	err := ts.Spawn(p)
	assert.Nil(t, err)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())
	// assert.Equal(t, 795, n)
	ts.Shutdown()
}

type Tstate struct {
	*test.Tstate
	jobRoot     string
	job         string
	nreducetask int
	tasks       *mr.Tasks
}

func newTstate(t1 *test.Tstate, jobRoot, app string) *Tstate {
	ts := &Tstate{}
	ts.jobRoot = jobRoot
	ts.Tstate = t1
	j, err := mr.ReadJobConfig(app)
	assert.Nil(t1.T, err, "Error ReadJobConfig: %v", err)
	job = j
	ts.nreducetask = job.Nreduce
	ts.job = rd.String(4)

	// If we don't remove all temp files (and there are many left over from
	// previous runs of the tests), ux may be very slow and cause the test to
	// hang during intialization. Using RmDir on ux is slow too, so just do this
	// directly through the os for now.
	os.RemoveAll(filepath.Join(sp.SIGMAHOME, "mr"))

	tasks, err := mr.InitCoordFS(ts.FsLib, ts.jobRoot, ts.job, ts.nreducetask)
	assert.Nil(t1.T, err, "Error InitCoordFS: %v", err)
	ts.tasks = tasks
	os.Remove(OUTPUT)

	return ts
}

// Returns true if comparison was successful (expected output == actual output)
func (ts *Tstate) compare() bool {
	cmd := exec.Command("sort", "gutenberg.txt.out")
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
	if assert.Equal(ts.T, len(b1), len(b2), "Output files have different length") {
		// Only do byte-by-byte comparison if output lengths are the same
		// (otherwise we just crowd the test output)
		return assert.Equal(ts.T, b1, b2, "Output files have different contents")
	}
	return false
}

func (ts *Tstate) checkJob(app string) bool {
	err := mr.MergeReducerOutput(ts.FsLib, ts.jobRoot, ts.job, OUTPUT, ts.nreducetask)
	assert.Nil(ts.T, err, "Merge output files: %v", err)
	if app == "mr-wc.yml" || app == "mr-ux-wc.yml" || app == MALICIOUS_APP {
		db.DPrintf(db.TEST, "checkJob %v", app)
		return ts.compare()
	}
	return true
}

func runN(t *testing.T, crashtask, crashcoord, crashschedd, crashprocq, crashux, maliciousMapper int, monitor bool) {
	var s3secrets *sp.SecretProto
	var err1 error
	// If running with malicious mappers, try to get restricted AWS secrets
	// before starting the system
	if maliciousMapper > 0 {
		s3secrets, err1 = auth.GetAWSSecrets(sp.AWS_S3_RESTRICTED_PROFILE)
		if !assert.Nil(t, err1, "Can't get secrets for aws profile %v: %v", sp.AWS_S3_RESTRICTED_PROFILE, err1) {
			return
		}
	}

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	var sc *sigmaclnt.SigmaClnt = t1.SigmaClnt
	runApp := app
	if maliciousMapper > 0 {
		db.DPrintf(db.ALWAYS, "Overriding MR app settting to run on restricted S3 bucket with malicious mapper: %v", MALICIOUS_APP)
		runApp = MALICIOUS_APP

		// Create a new sigma clnt
		pe := proc.NewAddedProcEnv(t1.ProcEnv())
		pe.SetPrincipal(sp.NewPrincipal(
			sp.TprincipalID("mr-restricted-principal"),
			pe.GetRealm(),
		))

		// Load restricted AWS secrets
		pe.SetSecrets(map[string]*sp.SecretProto{"s3": s3secrets})

		// Create a SigmaClnt with the more restricted principal.
		sc, err1 = sigmaclnt.NewSigmaClnt(pe)
		if assert.Nil(t, err1, "Err NewSigmaClnt: %v", err1) {
			defer sc.StopWatchingSrvs()
		}
	}

	jobRoot := mr.MRDIRTOP

	ts := newTstate(t1, jobRoot, runApp)

	err := ts.BootNode(1)
	assert.Nil(t, err, "BootProcd 1")

	err = ts.BootNode(1)
	assert.Nil(t, err, "BootProcd 2")

	sdc := scheddclnt.NewScheddClnt(sc.FsLib, sp.NOT_SET)
	if monitor {
		sdc.MonitorScheddStats(ts.ProcEnv().GetRealm(), time.Second)
		defer sdc.Done()
	}

	nmap, err := mr.PrepareJob(sc.FsLib, ts.tasks, ts.jobRoot, ts.job, job)
	assert.Nil(ts.T, err, "Err prepare job %v: %v", job, err)
	assert.NotEqual(ts.T, 0, nmap)

	cm := mr.StartMRJob(sc, ts.jobRoot, ts.job, job, mr.NCOORD, nmap, crashtask, crashcoord, MEM_REQ, true, maliciousMapper)

	crashchan := make(chan bool)
	l1 := &sync.Mutex{}
	for i := 0; i < crashschedd; i++ {
		// Sleep for a random time, then crash a server.
		go ts.CrashServer(sp.SCHEDDREL, (i+1)*CRASHSRV, l1, crashchan)
	}
	l2 := &sync.Mutex{}
	for i := 0; i < crashux; i++ {
		// Sleep for a random time, then crash a server.
		go ts.CrashServer(sp.UXREL, (i+1)*CRASHSRV, l2, crashchan)
	}
	l3 := &sync.Mutex{}
	for i := 0; i < crashprocq; i++ {
		// Sleep for a random time, then crash a server.
		go ts.CrashServer(sp.PROCQREL, (i+1)*CRASHSRV, l3, crashchan)
	}

	db.DPrintf(db.TEST, "WaitGroup")
	cm.WaitGroup()
	db.DPrintf(db.TEST, "Done WaitGroup")

	for i := 0; i < crashschedd+crashux+crashprocq; i++ {
		<-crashchan
	}

	db.DPrintf(db.TEST, "Check Job")
	ok := ts.checkJob(runApp)
	// Check that the malicious mapper didn't succeed (which would cause the
	// output files not to match)
	if !ok && maliciousMapper > 0 {
		assert.False(ts.T, true, "Output files don't match when running with malicious mapper. Suspected security authorization violation. Check error logs.")
	}
	db.DPrintf(db.TEST, "Done check Job")

	err = mr.PrintMRStats(ts.FsLib, ts.jobRoot, ts.job)
	assert.Nil(ts.T, err, "Error print MR stats: %v", err)

	db.DPrintf(db.TEST, "Cleanup tasks state")
	ts.tasks.Mft.Cleanup()
	ts.tasks.Rft.Cleanup()
	mr.CleanupMROutputs(ts.FsLib, mr.JobOut(job.Output, ts.job), mr.MapIntermediateDir(ts.job, job.Intermediate))
	db.DPrintf(db.TEST, "Done cleanup MR outputs")
	ts.Shutdown()
}

func TestMRJob(t *testing.T) {
	runN(t, 0, 0, 0, 0, 0, 0, true)
}

func TestMaliciousMapper(t *testing.T) {
	runN(t, 0, 0, 0, 0, 0, 500, true)
}

func TestCrashTaskOnly(t *testing.T) {
	runN(t, CRASHTASK, 0, 0, 0, 0, 0, false)
}

func TestCrashCoordOnly(t *testing.T) {
	runN(t, 0, CRASHCOORD, 0, 0, 0, 0, false)
}

func TestCrashTaskAndCoord(t *testing.T) {
	runN(t, CRASHTASK, CRASHCOORD, 0, 0, 0, 0, false)
}

func TestCrashSchedd1(t *testing.T) {
	runN(t, 0, 0, 1, 0, 0, 0, false)
}

func TestCrashSchedd2(t *testing.T) {
	N := 2
	runN(t, 0, 0, N, 0, 0, 0, false)
}

func TestCrashScheddN(t *testing.T) {
	N := 5
	runN(t, 0, 0, N, 0, 0, 0, false)
}

func TestCrashProcq1(t *testing.T) {
	runN(t, 0, 0, 0, 1, 0, 0, false)
}

func TestCrashProcq2(t *testing.T) {
	N := 2
	runN(t, 0, 0, 0, N, 0, 0, false)
}

func TestCrashProcqN(t *testing.T) {
	N := 5
	runN(t, 0, 0, 0, N, 0, 0, false)
}

func TestCrashUx1(t *testing.T) {
	N := 1
	runN(t, 0, 0, 0, 0, N, 0, false)
}

func TestCrashUx2(t *testing.T) {
	N := 2
	runN(t, 0, 0, 0, 0, N, 0, false)
}

func TestCrashUx5(t *testing.T) {
	N := 5
	runN(t, 0, 0, 0, 0, N, 0, false)
}

func TestCrashScheddProcqUx5(t *testing.T) {
	N := 5
	runN(t, 0, 0, N, N, N, 0, false)
}
