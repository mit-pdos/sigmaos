package fsux

import (
	"bufio"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	mr "sigmaos/apps/mr"
	mrapi "sigmaos/apps/mr/mr"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/path"
	"sigmaos/proc"
	uxclnt "sigmaos/proxy/ux/clnt"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
)

var fn string

const (
	FILESZ  = 50 * sp.MBYTE
	WRITESZ = 4096
)

func init() {
	fn = path.MarkResolve(filepath.Join(sp.UX, sp.ANY))
}

func TestCompile(t *testing.T) {
}

func TestRoot(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	dirents, err := ts.GetDir(fn)
	assert.Nil(t, err, "GetDir")

	assert.NotEqual(t, 0, len(dirents))

	ts.Shutdown()
}

func TestFile(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	d := []byte("hello")
	_, err := ts.PutFile(fn+"f", 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	d1, err := ts.GetFile(fn + "f")
	assert.Equal(t, string(d), string(d1))

	err = ts.Remove(fn + "f")
	assert.Equal(t, nil, err)

	ts.Shutdown()
}

func TestFileRPCClnt(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	d := []byte("hello")
	_, err := ts.PutFile(fn+"f", 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	// Use UX client to get the file via RPC
	uxClnt, err := uxclnt.NewUXClnt(ts.FsLib, fn)
	if !assert.Nil(t, err, "Error creating UX client: %v", err) {
		return
	}

	d1, err := uxClnt.GetFile("f")
	if !assert.Nil(t, err, "Error getting file via RPC: %v", err) {
		return
	}
	assert.Equal(t, string(d), string(d1), "File contents should match")

	err = ts.Remove(fn + "f")
	assert.Equal(t, nil, err)

	ts.Shutdown()
}

func TestDir(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	err := ts.MkDir(fn+"d1", 0777)
	assert.Equal(t, nil, err)
	d := []byte("hello")

	dirents, err := ts.GetDir(fn + "d1")
	assert.Nil(t, err, "GetDir")

	assert.Equal(t, 0, len(dirents))

	_, err = ts.PutFile(fn+"d1/f", 0777, sp.OWRITE, d)
	assert.Equal(t, nil, err)

	d1, err := ts.GetFile(fn + "d1/f")
	assert.Equal(t, string(d), string(d1))

	err = ts.Remove(fn + "d1/f")
	assert.Equal(t, nil, err)

	err = ts.Remove(fn + "d1")
	assert.Equal(t, nil, err)

	ts.Shutdown()
}

// TestMapperZeroOutputShardRace reproduces the MR bug in which an MR mapper
// that emits nothing (e.g., grep over a split with no matches) never
// synchronized with its asynchronous initOutput goroutine — only Emit did — so
// DoMap could report its shard names and the proc could exit while the shard
// files were still being created on UX. The proc's exit detaches its session,
// clunking its fids and killing the in-flight creates (the fsuxd-side
// fingerprint is FidMap.Update failing on the clunked fid,
// spproto/srv/fid/fidmap.go), so the reported shard files may never exist. A
// reducer that later reads them fails and exits with RESTART.
//
// The test mimics the zero-output mapper faithfully: run DoMap, then
// immediately close the client (as the proc's exit would), and assert the
// invariant the reducer depends on: every shard in the mapper's reported
// output bin exists.
func TestMapperZeroOutputShardRace(t *testing.T) {
	const (
		NTRIAL  = 10
		NREDUCE = 8
	)
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	inPn := fn + "mr-race-in.txt"
	d := []byte("a b c\n")
	_, err := ts.PutFile(inPn, 0777, sp.OWRITE, d)
	if !assert.Nil(t, err, "PutFile: %v", err) {
		return
	}
	bin, err := json.Marshal([]mrapi.Split{{File: inPn, Offset: 0, Length: sp.Tlength(len(d))}})
	if !assert.Nil(t, err, "Marshal: %v", err) {
		return
	}

	// A mapper that emits nothing, like grep over a split with no matches
	zeroMap := func(string, *bufio.Scanner, mrapi.EmitT) error { return nil }

	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	p := &perf.Perf{}
	for i := 0; i < NTRIAL; i++ {
		sc, err := sigmaclnt.NewSigmaClnt(pe)
		if !assert.Nil(t, err, "NewSigmaClnt: %v", err) {
			break
		}
		job := fmt.Sprintf("mr-uxrace-%d", i)
		m, err := mr.NewMapper(sc, zeroMap, nil, "name/mr/", job, p, NREDUCE, 8192, 40, string(bin), "name/ux/~local/mr-intermediate", false, false, 0, nil)
		if !assert.Nil(t, err, "NewMapper: %v", err) {
			break
		}
		_, _, obin, err := m.DoMap()
		if !assert.Nil(t, err, "DoMap: %v", err) {
			break
		}
		// The mapper proc exits right after DoMap; closing the client
		// detaches its sessions the same way, killing any shard creates
		// still in flight.
		sc.Close()
		// The reducer's view: every shard the mapper reported must exist
		for _, s := range obin {
			_, err := ts.Stat(s.File)
			assert.Nil(t, err, "reported shard %v missing (trial %d): %v", s.File, i, err)
		}
	}
}

func writer(t *testing.T, ch chan struct{}, ch2 chan struct{}, pe *proc.ProcEnv, idx int) {
	fsl, err := sigmaclnt.NewFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
	assert.Nil(t, err)
	fn := filepath.Join(sp.UX, sp.ANY, "file-"+string(pe.GetPrincipal().GetID())+"-"+strconv.Itoa(idx))
	stop := false
	ncrash := 0
	for !stop {
		select {
		case <-ch:
			stop = true
		default:
			if err := fsl.Remove(fn); serr.IsErrorSession(err) {
				ncrash += 1
				break
			}
			w, err := fsl.CreateBufWriter(fn, 0777)
			if err != nil {
				assert.True(t, serr.IsErrorSession(err), "Err code %v", err)
				ncrash += 1
				break
			}
			db.DPrintf(db.TEST, "created %v %d\n", fn, ncrash)
			buf := test.NewBuf(WRITESZ)
			if err := test.Writer(t, w, buf, FILESZ); err != nil {
				ncrash += 1
				break
			}
			if err := w.Close(); err != nil {
				assert.True(t, serr.IsErrorSession(err))
				ncrash += 1
				break
			}
		}
	}
	assert.True(t, ncrash >= 1)
	fsl.Remove(fn)
	fsl.Close()
	ch2 <- struct{}{}
}

func TestWriteCrash5x20(t *testing.T) {
	const (
		N        = 5
		NCRASH   = 5
		CRASHSRV = 1000000
		T        = 2000
	)

	fn := sp.NAMED + fmt.Sprintf("crashux%d.sem", 0)
	e0 := crash.NewEventPath(crash.UX_CRASH, 0, float64(1.0), fn)
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e0))
	assert.Nil(t, err)

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	ch := make(chan struct{})
	ch2 := make(chan struct{})
	for i := 0; i < N; i++ {
		pe := proc.NewAddedProcEnv(ts.ProcEnv())
		go writer(ts.T, ch, ch2, pe, i)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < NCRASH; i++ {
			fn = sp.NAMED + fmt.Sprintf("crashux%d.sem", i+1)
			e1 := crash.NewEventPath(crash.UX_CRASH, 0, float64(1.0), fn)
			err := crash.SetSigmaFail(crash.NewTeventMapOne(e1))
			assert.Nil(t, err)
			ts.CrashServer(e0, e1, sp.UXREL)
			e0 = e1
			time.Sleep(T * time.Millisecond)
		}
	}()
	wg.Wait()

	for i := 0; i < N; i++ {
		ch <- struct{}{}
	}

	for i := 0; i < N; i++ {
		<-ch2
	}

	ts.Shutdown()
}
