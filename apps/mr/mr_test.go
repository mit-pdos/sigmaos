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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"

	"sigmaos/apps/mr"
	"sigmaos/apps/mr/chunkreader"
	api "sigmaos/apps/mr/mr"
	mrscanner "sigmaos/apps/mr/scanner"
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/proc"
	mschedclnt "sigmaos/sched/msched/clnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/auth"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
	rd "sigmaos/util/rand"

	// "sigmaos/sigmasrv/stats"
	"sigmaos/apps/mr/grep"
	"sigmaos/apps/mr/wc"
	"sigmaos/test"
)

const (
	OUTPUT        = "/tmp/par-mr.out"
	MALICIOUS_APP = "mr-wc-restricted.yml"

	// time interval (ms) for when a failure might happen. If too
	// frequent and they don't finish ever. XXX determine
	// dynamically
	CRASHREDUCE = 150
	CRASHMAP    = 400
	CRASHCOORD  = 1000
	// CRASHSRV    = 1000
	CRASHSRV = 500
	MEM_REQ  = 1000
)

var app string // yaml app file
var nmap int
var job *mr.Job
var timeout time.Duration

var coordEv *crash.TeventMap
var mapEv *crash.TeventMap
var reduceEv *crash.TeventMap

func init() {
	flag.StringVar(&app, "app", "mr-wc.yml", "application")
	flag.IntVar(&nmap, "nmap", 1, "number of mapper threads")
	flag.DurationVar(&timeout, "mr-timeout", 0, "timeout")

	e0 := crash.NewEventStart(crash.MRMAP_CRASH, 100, CRASHMAP, 0.33)
	e1 := crash.NewEventStart(crash.MRMAP_PARTITION, 100, CRASHMAP, 0.33)
	mapEv = crash.NewTeventMapOne(e0)
	mapEv.Insert(e1)
	e0 = crash.NewEventStart(crash.MRREDUCE_CRASH, 0, CRASHREDUCE, 0.33)
	e1 = crash.NewEventStart(crash.MRREDUCE_PARTITION, 0, CRASHREDUCE, 0.33)
	reduceEv = crash.NewTeventMapOne(e0)
	reduceEv.Insert(e1)
	e0 = crash.NewEventStart(crash.MRCOORD_CRASH, 100, CRASHCOORD, 0.33)
	e1 = crash.NewEventStart(crash.MRCOORD_PARTITION, 100, CRASHCOORD, 0.33)
	coordEv = crash.NewTeventMapOne(e0)
	coordEv.Insert(e1)
}

func TestCompile(t *testing.T) {
}

func TestHash(t *testing.T) {
	assert.Equal(t, 0, mr.Khash([]byte("LEAGUE"))%8)
	assert.Equal(t, 0, mr.Khash([]byte("Abbots"))%8)
	assert.Equal(t, 0, mr.Khash([]byte("yes"))%8)
	assert.Equal(t, 7, mr.Khash([]byte("absently"))%8)
}

func TestWordSpanningChunk(t *testing.T) {
	const (
		CKSZ    = 8
		SPLITSZ = sp.MBYTE
		LINESZ  = 65536
		WORDSZ  = 20
		NWORD   = 7777 // According to TestSeqWc
		WC      = "/tmp/sigmaos/pg-dorian_gray.txt.wc"
	)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	fn := filepath.Join("name/s3/" + sp.LOCAL + "/9ps3/gutenberg/pg-dorian_gray.txt")
	fn, ok := sp.S3ClientPath(fn)
	assert.True(t, ok)
	s := &api.Split{fn, 0, SPLITSZ}
	ts.MountS3PathClnt()

	pfr, err := ts.OpenParallelFileReader(s.File, s.Offset, s.Length)
	assert.Nil(t, err)

	p, err := perf.NewPerf(ts.ProcEnv(), perf.MRMAPPER)
	assert.Nil(t, err)

	ckr := chunkreader.NewChunkReader(LINESZ, WORDSZ, wc.Reduce, p)
	n, err := ckr.ReadChunks(pfr, s, wc.Map)
	assert.Nil(t, err)

	kvmap := ckr.KVMap()

	db.DPrintf(db.TEST, "bytes %d words %d", n, kvmap.Len())

	assert.Equal(t, NWORD, kvmap.Len())

	file, err := os.Create(WC)
	assert.Nil(t, err)
	defer file.Close()
	w := bufio.NewWriter(file)
	defer w.Flush()
	kvmap.Emit(wc.Reduce, func(k []byte, v string) error {
		b := fmt.Sprintf("%s\t%v\n", string(k), v)
		_, err := w.Write([]byte(b))
		return err
	})
	p.Done()

	ts.Shutdown()
}

type Tdata map[string]uint64

func wcline(n int, line string, data Tdata, sbc *mrscanner.ScanByteCounter) (int, error) {
	scanner := bufio.NewScanner(strings.NewReader(line))
	scanner.Split(sbc.ScanWords)
	cnt := 0
	for scanner.Scan() {
		w := scanner.Text()
		if _, ok := data[w]; !ok {
			data[w] = uint64(0)
		}
		data[w] += 1
		cnt++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return cnt, nil
}

func TestSeqWc(t *testing.T) {
	const (
		LOCALINPUT = "/tmp/enwiki-1G"
		HOSTTMP    = "/tmp/sigmaos/"
		F          = "pg-dorian_gray.txt"
		INPUT      = "../../input/" + F
		// INPUT = LOCALINPUT
		OUT = HOSTTMP + F + ".out"
	)

	file, err := os.Open(INPUT)
	assert.Nil(t, err)
	defer file.Close()
	r := bufio.NewReader(file)
	start := time.Now()
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 2097152)
	scanner.Buffer(buf, cap(buf))
	data := make(Tdata, 0)
	p, err := perf.NewPerf(proc.NewTestProcEnv(sp.ROOTREALM, nil, nil, sp.NO_IP, sp.NO_IP, "", false, false), perf.SEQWC)
	assert.Nil(t, err)
	sbc := mrscanner.NewScanByteCounter(p)
	for scanner.Scan() {
		l := scanner.Text()
		if len(l) > 0 {
			_, err := wcline(0, l, data, sbc)
			assert.Nil(t, err)
		}
	}
	err = scanner.Err()
	assert.Nil(t, err)
	db.DPrintf(db.ALWAYS, "seqwc %v %v %v", INPUT, time.Since(start), OUT)
	file, err = os.Create(OUT)
	assert.Nil(t, err)
	defer file.Close()
	w := bufio.NewWriter(file)
	defer w.Flush()
	for k, v := range data {
		b := fmt.Sprintf("%s\t%d\n", k, v)
		_, err := w.Write([]byte(b))
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
	job, err1 = mr.ReadJobConfig(filepath.Join("job-descriptions", app))
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

	p, err := perf.NewPerf(proc.NewTestProcEnv(sp.ROOTREALM, nil, nil, sp.NO_IP, sp.NO_IP, "", false, false), perf.MRMAPPER)
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
		m, err := mr.NewMapper(sc, mapper, reducer, ts.jobRoot, ts.job, p, job.Nreduce, job.Linesz, job.Wordsz, string(bin), job.Intermediate)
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
		db.DPrintf(db.ALWAYS, "%s: reduce in %v out %v tot %v %vms (%s)\n", res.Task, humanize.Bytes(uint64(res.In)), humanize.Bytes(uint64(res.Out)), test.Mbyte(res.In+res.Out), res.MsInner, test.TputStr(res.In+res.Out, res.MsInner))
	}

	if app == "mr-wc.yml" || app == "mr-ux-wc.yml" {
		ts.checkJob(app)
	}

	p.Done()
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
	j, err := mr.ReadJobConfig(filepath.Join("job-descriptions", app))
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

func crashSemPn(l crash.Tselector, i int) string {
	fn := sp.NAMED + fmt.Sprintf("%v-%d.sem", l, i)
	return fn
}

func (ts *Tstate) crashServers(srv string, l crash.Tselector, em *crash.TeventMap, n int) {
	e0, ok := em.Lookup(l)
	assert.True(ts.T, ok)
	for i := 0; i < n; i++ {
		time.Sleep(CRASHSRV * time.Millisecond)
		e1 := crash.NewEventPath(string(l), 0, 1.0, crashSemPn(l, i+1))
		ts.CrashServer(e0, e1, srv)
		e0 = e1
	}
}

func (ts *Tstate) collectStats(stati []*procgroupmgr.ProcStatus) (int, mr.Stat) {
	mrst := mr.Stat{}
	nrestart := 0
	for _, st := range stati {
		nrestart += st.Nrestart
		db.DPrintf(db.TEST, "grpmgr stat: %v", st)
		if st.IsStatusOK() {
			// if st.Status != nil && st.IsStatusOK() {
			t := mr.Stat{}
			err := mapstructure.Decode(st.Data(), &t)
			assert.Nil(ts.T, err)
			if t.Nmap > 0 || t.Nreduce > 0 {
				mrst = t
			}
			if t.Ntask > mrst.Ntask {
				mrst.Ntask = t.Ntask
			}
		}
	}
	return nrestart, mrst
}

func runN(t *testing.T, em *crash.TeventMap, srvs map[string]crash.Tselector, maliciousMapper int, monitor bool) (int, int, *mr.Stat) {
	var s3secrets *sp.SecretProto
	var err1 error
	// If running with malicious mappers, try to get restricted AWS secrets
	// before starting the system
	if maliciousMapper > 0 {
		s3secrets, err1 = auth.GetAWSSecrets(sp.AWS_S3_RESTRICTED_PROFILE)
		if !assert.Nil(t, err1, "Can't get secrets for aws profile %v: %v", sp.AWS_S3_RESTRICTED_PROFILE, err1) {
			return 0, 0, nil
		}
	}

	// XXX maybe in pe
	err := crash.SetSigmaFail(em)
	assert.Nil(t, err)

	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return 0, 0, nil
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

	// Start more nodes to run mappers/reducers in parallel (except
	// for crash tests).
	if len(srvs) == 0 {
		err = ts.BootNode(1)
		assert.Nil(t, err, "BootProcd 1")
		err = ts.BootNode(1)
		assert.Nil(t, err, "BootProcd 2")
	}

	sdc := mschedclnt.NewMSchedClnt(sc.FsLib, sp.NOT_SET)
	if monitor {
		sdc.MonitorMSchedStats(ts.ProcEnv().GetRealm(), time.Second)
		defer sdc.Done()
	}

	nmap, err := mr.PrepareJob(sc.FsLib, ts.tasks, ts.jobRoot, ts.job, job)
	assert.Nil(ts.T, err, "Err prepare job %v: %v", job, err)
	assert.NotEqual(ts.T, 0, nmap)

	cm := mr.StartMRJob(sc, ts.jobRoot, ts.job, job, nmap, MEM_REQ, maliciousMapper)

	var wg sync.WaitGroup
	for k, v := range srvs {
		wg.Add(1)
		go func() {
			ts.crashServers(k, v, em, 1)
			wg.Done()
		}()
	}
	wg.Wait()

	db.DPrintf(db.TEST, "WaitGroup")

	stati := cm.WaitGroup()
	nrestart, mrst := ts.collectStats(stati)

	db.DPrintf(db.TEST, "Done WaitGroup %d %v", nrestart, &mrst)

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
	return nmap + ts.nreducetask, nrestart, &mrst
}

// if f returns true, repeat test
func repeatTest(t *testing.T, f func() bool) {
	ok := false
	for i := 0; i < 10; i++ {
		if !f() {
			ok = true
			break
		}
	}
	assert.True(t, ok)
}

func TestMRJob(t *testing.T) {
	n, _, st := runN(t, nil, nil, 0, true)
	assert.Equal(t, n, st.Ntask)
	assert.Equal(t, 0, st.Nfail)
}

func TestMaliciousMapper(t *testing.T) {
	runN(t, nil, nil, 500, true)
}

func TestCrashMapperOnly(t *testing.T) {
	_, _, st := runN(t, mapEv, nil, 0, false)
	assert.True(t, st.Nfail > 0)
}

func TestCrashReducerOnlyCrash(t *testing.T) {
	repeatTest(t, func() bool {
		_, _, st := runN(t, reduceEv.Filter(crash.MRREDUCE_CRASH), nil, 0, false)
		return st.Nfail == 0
	})
}

func TestCrashReducerOnlyPartition(t *testing.T) {
	repeatTest(t, func() bool {
		_, _, st := runN(t, reduceEv.Filter(crash.MRREDUCE_PARTITION), nil, 0, false)
		return st.Nfail == 0
	})
}

func TestCrashReducerOnlyBoth(t *testing.T) {
	_, _, st := runN(t, reduceEv, nil, 0, false)
	assert.True(t, st.Nfail > 0)
}

func TestCrashCoordOnly(t *testing.T) {
	_, nr, _ := runN(t, coordEv, nil, 0, false)
	assert.True(t, nr > mr.NCOORD)
}

func TestCrashTaskAndCoord(t *testing.T) {
	em := crash.NewTeventMap()
	em.Merge(mapEv)
	em.Merge(reduceEv)
	em.Merge(coordEv)
	ntask, nr, st := runN(t, em, nil, 0, false)
	assert.True(t, nr > mr.NCOORD)
	assert.True(t, st.Ntask > ntask)
}

func TestCrashUx1(t *testing.T) {
	e0 := crash.NewEventPath(crash.UX_CRASH, 0, 1.0, crashSemPn(crash.UX_CRASH, 0))
	srvs := make(map[string]crash.Tselector)
	srvs[sp.UXREL] = crash.UX_CRASH
	ntask, _, st := runN(t, crash.NewTeventMapOne(e0), srvs, 0, false)
	assert.True(t, st.Ntask > ntask || st.Nfail > 0)
}

func TestCrashMSched1(t *testing.T) {
	e0 := crash.NewEventPath(crash.MSCHED_CRASH, CRASHCOORD, 1.0, crashSemPn(crash.MSCHED_CRASH, 0))
	srvs := make(map[string]crash.Tselector)
	srvs[sp.MSCHEDREL] = crash.MSCHED_CRASH
	ntask, _, st := runN(t, crash.NewTeventMapOne(e0), srvs, 0, false)
	assert.True(t, st.Ntask > ntask || st.Nfail > 0)
}

func TestCrashBESched1(t *testing.T) {
	e0 := crash.NewEventPath(crash.BESCHED_CRASH, CRASHCOORD, 1.0, crashSemPn(crash.BESCHED_CRASH, 0))
	srvs := make(map[string]crash.Tselector)
	srvs[sp.BESCHEDREL] = crash.BESCHED_CRASH
	ntask, _, st := runN(t, crash.NewTeventMapOne(e0), srvs, 0, false)
	assert.True(t, st.Ntask >= ntask || st.Nfail >= 0)
}

func TestCrashProcd1(t *testing.T) {
	e0 := crash.NewEventPath(crash.PROCD_CRASH, CRASHCOORD, 1.0, crashSemPn(crash.PROCD_CRASH, 0))
	srvs := make(map[string]crash.Tselector)
	srvs[sp.PROCDREL] = crash.PROCD_CRASH
	ntask, _, st := runN(t, crash.NewTeventMapOne(e0), srvs, 0, false)
	assert.True(t, st.Ntask > ntask || st.Nfail > 0)
}

func TestCrashMSchedBESchedUx1(t *testing.T) {
	e := crash.NewEventPath(crash.UX_CRASH, 0, 1.0, crashSemPn(crash.UX_CRASH, 0))
	em := crash.NewTeventMapOne(e)
	e = crash.NewEventPath(crash.MSCHED_CRASH, CRASHCOORD, 1.0, crashSemPn(crash.MSCHED_CRASH, 0))
	em.Insert(e)
	srvs := make(map[string]crash.Tselector)
	srvs[sp.UXREL] = crash.UX_CRASH
	srvs[sp.MSCHEDREL] = crash.MSCHED_CRASH
	ntask, _, st := runN(t, em, srvs, 0, false)
	assert.True(t, st.Ntask > ntask || st.Nfail > 0)
}
