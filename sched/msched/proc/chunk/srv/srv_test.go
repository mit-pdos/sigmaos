package srv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/sched/msched/proc/chunk"
	chunkclnt "sigmaos/sched/msched/proc/chunk/clnt"
	chunksrv "sigmaos/sched/msched/proc/chunk/srv"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	PROG = "sleeper-v1.0"
	PATH = "name/ux/" + sp.LOCAL + "/bin/user/common/"
)

type Tstate struct {
	*test.Tstate
	ckclnt *chunkclnt.ChunkClnt
	srvs   []string
	bins   *chunkclnt.BinPaths
}

func newTstate(t *testing.T, n int) *Tstate {
	ts := &Tstate{
		bins: chunkclnt.NewBinPaths(),
	}
	s, err := test.NewTstateAll(t)
	assert.Nil(t, err)
	ts.Tstate = s

	err = s.BootNode(n)
	assert.Nil(t, err, "Boot node: %v", err)

	ckclnt := chunkclnt.NewChunkClnt(ts.FsLib, true)
	srvs, err := ckclnt.WaitTimedGetEntriesN(n + 1)
	assert.Nil(t, err)

	ts.srvs = srvs
	ts.ckclnt = ckclnt

	db.DPrintf(db.TEST, "Chunksrvs  %v", ts.srvs)

	for _, srv := range srvs {
		pn := chunksrv.PathHostKernelRealm(srv, sp.ROOTREALM)
		os.Mkdir(pn, 0700)
	}
	return ts
}

func (ts *Tstate) shutdown() {
	ts.ckclnt.StopWatching()
	ts.Shutdown()
}

func (ts *Tstate) check(srv string, st *sp.Tstat) {
	pn := chunksrv.PathHostKernelRealm(srv, sp.ROOTREALM)
	pn = filepath.Join(pn, PROG)
	fi, err := os.Stat(pn)
	assert.Nil(ts.T, err)
	assert.Equal(ts.T, st.Tlength(), sp.Tlength(fi.Size()))
}

func isExpected(path string, expect []string) bool {
	for _, e := range expect {
		if e == path {
			return true
		}
	}
	return false
}

func (ts *Tstate) fetch(srv string, paths []string, expect []string) int {
	pid := ts.ProcEnv().GetPID()
	secrets := ts.ProcEnv().GetSecrets()["s3"]
	pnsrv := chunk.ChunkdPath(srv)
	st, path, err := ts.ckclnt.GetFileStat(srv, PROG, pid, sp.ROOTREALM, secrets, paths, nil)
	if !assert.Nil(ts.T, err, "Err GetFileStat: %v", err) {
		return 0
	}
	assert.True(ts.T, isExpected(path, expect))

	n := (st.Tlength() / sp.Tlength(sp.Conf.Chunk.CHUNK_SZ)) + 1
	l := 0
	h := int(n - 1)
	nlocal := 0
	for i := 0; i < int(n); i++ {
		ck := 0
		if i%2 == 0 {
			ck = l
			l++
		} else {
			ck = h
			h--
		}
		sz, path, err := ts.ckclnt.Fetch(srv, PROG, pid, sp.ROOTREALM, secrets, ck, st.Tsize(), paths, ts.ProcEnv().GetNamedEndpointProto())
		db.DPrintf(db.TEST, "ck %d(%d) srv %v path %v expect %v nlocal %d", ck, n, srv, path, expect, nlocal)
		assert.Nil(ts.T, err, "err %v", err)
		assert.True(ts.T, sz > 0 && sz <= sp.Tsize(sp.Conf.Chunk.CHUNK_SZ))

		// chunkd prefetches and may return a chunk from srv
		isLocal := isExpected(path, []string{pnsrv})
		if isLocal {
			nlocal += 1
		}
		assert.True(ts.T, isExpected(path, expect) || isLocal)
	}

	ts.bins.SetBinKernelID(PROG, srv)

	ts.check(srv, st)
	return nlocal
}

func TestCompile(t *testing.T) {
}

func TestFetchOrigin(t *testing.T) {
	ts := newTstate(t, 0)
	ts.fetch(ts.srvs[0], []string{PATH}, []string{PATH})
	ts.shutdown()
}

func TestFetchCache(t *testing.T) {
	ts := newTstate(t, 0)

	ts.fetch(ts.srvs[0], []string{PATH}, []string{PATH})
	ts.fetch(ts.srvs[0], []string{PATH}, []string{chunk.ChunkdPath(ts.srvs[0])})

	ts.shutdown()
}

func TestFetchChunkd(t *testing.T) {
	ts := newTstate(t, 1)

	ts.fetch(ts.srvs[0], []string{PATH}, []string{PATH})

	kid, ok := ts.bins.GetBinKernelID(PROG)
	assert.True(ts.T, ok)

	pn := chunk.ChunkdPath(kid)
	ts.fetch(ts.srvs[0], []string{pn}, []string{pn})

	ts.shutdown()
}

func TestFetchPath(t *testing.T) {
	ts := newTstate(t, 1)

	// fetch through chunkd1 so that it has it cached
	pn1 := chunk.ChunkdPath(ts.srvs[1])
	ts.fetch(ts.srvs[1], []string{pn1, PATH}, []string{PATH})

	// fetch through chunkd 0 with chunkd1 in search path,
	// so data should come from chunkd1
	n := ts.fetch(ts.srvs[0], []string{pn1, PATH}, []string{pn1})
	assert.True(t, n == 1)

	ts.shutdown()
}

func TestFetchConcur(t *testing.T) {
	const N = 10
	ts := newTstate(t, 1)

	ch := make(chan int)
	for i := 0; i < N; i++ {
		go func(i int) {
			pn0 := chunk.ChunkdPath(ts.srvs[0])
			pn1 := chunk.ChunkdPath(ts.srvs[1])
			if i%2 == 0 {
				ts.fetch(ts.srvs[0], []string{pn1, PATH}, []string{pn0, pn1, PATH})
			} else {
				ts.fetch(ts.srvs[1], []string{pn0, PATH}, []string{pn0, pn1, PATH})
			}
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
	}
	ts.shutdown()
}
