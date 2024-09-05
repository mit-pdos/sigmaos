package mr_test

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	// "io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	"sigmaos/test"
	"sigmaos/wc"
)

const (
	OUTPUT        = "/tmp/par-mr.out"
	MALICIOUS_APP = "mr-wc-restricted.yml"
	LOCALINPUT    = "/tmp/enwiki-2G"

	// time interval (ms) for when a failure might happen. If too
	// frequent and they don't finish ever. XXX determine
	// dynamically
	CRASHTASK  = 3000
	CRASHCOORD = 6000
	CRASHSRV   = 1000000
	MEM_REQ    = 1000
)

var app string // yaml app file
var job *mr.Job

func init() {
	flag.StringVar(&app, "app", "mr-wc.yml", "application")
}

func TestCompile(t *testing.T) {
}

func TestHash(t *testing.T) {
	assert.Equal(t, 0, mr.Khash("LEAGUE")%8)
	assert.Equal(t, 0, mr.Khash("Abbots")%8)
	assert.Equal(t, 0, mr.Khash("yes")%8)
	assert.Equal(t, 7, mr.Khash("absently")%8)
}

func TestWordCount(t *testing.T) {
	const (
		HOSTTMP = "/tmp/sigmaos"
		F       = "gutenberg.txt"
		INPUT   = "../input/" + F
		// INPUT   = LOCALINPUT
		OUT = HOSTTMP + F + ".out"
	)

	file, err := os.Open(INPUT)
	assert.Nil(t, err)
	defer file.Close()
	r := bufio.NewReader(file)
	rdr, err := readahead.NewReaderSize(r, 4, sp.BUFSZ)
	assert.Nil(t, err, "Err reader: %v", err)
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

func TestMapperAlone(t *testing.T) {
	const (
		//SPLITSZ   =  64 * sp.KBYTE
		SPLITSZ   = 10 * sp.MBYTE
		REDUCEIN  = "name/ux/~local/test-reducer-in.txt"
		REDUCEOUT = "name/ux/~local/test-reducer-out.txt"
	)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	p, err := perf.NewPerf(proc.NewTestProcEnv(sp.ROOTREALM, nil, nil, sp.NO_IP, sp.NO_IP, "", false, false, false), perf.MRMAPPER)
	assert.Nil(t, err)

	ts.Remove(REDUCEIN)
	ts.Remove(REDUCEOUT)

	job, err1 = mr.ReadJobConfig(app) // or --app mr-ux-wiki1G.yml
	assert.Nil(t, err1, "Error ReadJobConfig: %v", err1)
	job.Nreduce = 1

	if strings.HasPrefix(job.Input, sp.UX) {
		file := filepath.Base(LOCALINPUT)
		err := ts.MkDir(job.Input, 0777)
		assert.Nil(t, err, "MkDir err %v", err)
		err = ts.UploadFile(LOCALINPUT, filepath.Join(job.Input, file))
		assert.Nil(t, err, "UploadFile err %v", err)
	}

	bins, err := mr.NewBins(ts.FsLib, job.Input, sp.Tlength(job.Binsz), SPLITSZ)
	assert.Nil(t, err, "Err NewBins %v", err)
	m, err := mr.NewMapper(ts.SigmaClnt, wc.Map, "test", p, job.Nreduce, job.Linesz, "nobin", "nointout", true)
	assert.Nil(t, err, "NewMapper %v", err)
	err = m.InitWrt(0, REDUCEIN)
	assert.Nil(t, err)

	start := time.Now()
	nin := sp.Tlength(0)
	for _, b := range bins {
		for _, s := range b {
			n, err := m.DoSplit(&s)
			if err != nil {
				db.DFatalf("DoSplit err %v", err)
			}
			nin += n
		}
	}
	nout, err := m.CloseWrt()
	if err != nil {
		db.DFatalf("CloseWrt err %v", err)
	}

	db.DPrintf(db.ALWAYS, "%s: in %s out %s tot %s %vms (%s)\n", "map", humanize.Bytes(uint64(nin)), humanize.Bytes(uint64(nout)), humanize.Bytes(uint64(test.Mbyte(nin+nout))), time.Since(start).Milliseconds(), test.TputStr(nin+nout, time.Since(start).Milliseconds()))

	// data := make(map[string]int, 0)
	// rdr, err := ts.OpenAsyncReader(REDUCEIN, 0)
	// assert.Nil(t, err)
	// for {
	// 	var kv mr.KeyValue
	// 	if err := mr.DecodeKV(rdr, &kv); err != nil {
	// 		if err == io.EOF {
	// 			break
	// 		}
	// 		assert.Nil(t, err)
	// 	}
	// 	if _, ok := data[kv.Key]; !ok {
	// 		data[kv.Key] = 0
	// 	}
	// 	data[kv.Key] += 1
	// }

	// wrt, err := ts.CreateAsyncWriter(REDUCEOUT, 0777, sp.OWRITE)
	// assert.Nil(t, err, "Err createAsynchWriter: %v", err)
	// for k, v := range data {
	// 	b := fmt.Sprintf("%s\t%d\n", k, v)
	// 	_, err := wrt.Write([]byte(b))
	// 	assert.Nil(t, err, "Err Write: %v", err)
	// }
	// if err == nil {
	// 	wrt.Close()
	// }

	// data1 := make(seqwc.Tdata)
	// sbc := mr.NewScanByteCounter(p)
	// _, _, err = seqwc.WcData(ts.FsLib, job.Input, data1, sbc)
	// assert.Nil(t, err)
	// assert.Equal(t, len(data1), len(data))

	// // for k, v := range data1 {
	// // 	if v1, ok := data[k]; !ok {
	// // 		log.Printf("error: k %s missing\n", k)
	// // 	} else {
	// // 		if uint64(len(v1)) != v {
	// // 			log.Printf("error: %s: %v != %v\n", k, v, v1)
	// // 		}
	// // 	}
	// // }

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
	job         string
	nreducetask int
	tasks       *mr.Tasks
}

func newTstate(t1 *test.Tstate, app string) *Tstate {
	ts := &Tstate{}
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

	tasks, err := mr.InitCoordFS(ts.FsLib, ts.job, ts.nreducetask)
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
	err := mr.MergeReducerOutput(ts.FsLib, ts.job, OUTPUT, ts.nreducetask)
	assert.Nil(ts.T, err, "Merge output files: %v", err)
	if app == "mr-wc.yml" || app == MALICIOUS_APP {
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
	ts := newTstate(t1, runApp)

	err := ts.BootNode(1)
	assert.Nil(t, err, "BootProcd 1")

	err = ts.BootNode(1)
	assert.Nil(t, err, "BootProcd 2")

	sdc := scheddclnt.NewScheddClnt(sc.FsLib)
	if monitor {
		sdc.MonitorScheddStats(ts.ProcEnv().GetRealm(), time.Second)
		defer sdc.Done()
	}

	nmap, err := mr.PrepareJob(sc.FsLib, ts.tasks, ts.job, job)
	assert.Nil(ts.T, err, "Err prepare job %v: %v", job, err)
	assert.NotEqual(ts.T, 0, nmap)

	cm := mr.StartMRJob(sc, ts.job, job, mr.NCOORD, nmap, crashtask, crashcoord, MEM_REQ, true, maliciousMapper)

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

	err = mr.PrintMRStats(ts.FsLib, ts.job)
	assert.Nil(ts.T, err, "Error print MR stats: %v", err)

	db.DPrintf(db.TEST, "Cleanup MR outputs")
	ts.tasks.Mft.Cleanup()
	ts.tasks.Rft.Cleanup()
	mr.CleanupMROutputs(ts.FsLib, job.Output, job.Intermediate)
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
