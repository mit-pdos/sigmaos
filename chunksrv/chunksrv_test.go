package chunksrv_test

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/chunk"
	"sigmaos/chunkclnt"
	"sigmaos/chunksrv"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	PROG = "sleeper"
	PATH = "name/ux/~local/bin/user/common/"
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

	ckclnt := chunkclnt.NewChunkClnt(ts.FsLib)
	srvs, err := ckclnt.GetSrvs()
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

func (ts *Tstate) check(srv string, st *sp.Stat) {
	pn := chunksrv.PathHostKernelRealm(srv, sp.ROOTREALM)
	pn = path.Join(pn, PROG)
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

func (ts *Tstate) fetch(srv string, paths []string, expect []string) {
	pid := ts.ProcEnv().GetPID()

	st, path, err := ts.ckclnt.GetFileStat(srv, PROG, pid, sp.ROOTREALM, paths)
	assert.Nil(ts.T, err)
	assert.True(ts.T, isExpected(path, expect))

	n := (st.Tlength() / chunk.CHUNKSZ) + 1
	l := 0
	h := int(n - 1)
	for i := 0; i < int(n); i++ {
		ck := 0
		if i%2 == 0 {
			ck = l
			l++
		} else {
			ck = h
			h--
		}
		sz, path, err := ts.ckclnt.Fetch(srv, PROG, pid, sp.ROOTREALM, ck, st.Tsize(), paths)
		db.DPrintf(db.TEST, "path %v", path)
		assert.Nil(ts.T, err, "err %v", err)
		assert.True(ts.T, sz > 0 && sz <= chunk.CHUNKSZ)
		assert.True(ts.T, isExpected(path, expect))
	}

	ts.bins.SetBinKernelID(PROG, srv)

	ts.check(srv, st)
}

func TestFetchOrigin(t *testing.T) {
	ts := newTstate(t, 0)
	ts.fetch(ts.srvs[0], []string{PATH}, []string{PATH})
	ts.Shutdown()
}

func TestFetchCache(t *testing.T) {
	ts := newTstate(t, 0)

	ts.fetch(ts.srvs[0], []string{PATH}, []string{PATH})
	ts.fetch(ts.srvs[0], []string{PATH}, []string{chunk.ChunkdPath(ts.srvs[0])})

	ts.Shutdown()
}

func TestFetchChunkd(t *testing.T) {
	ts := newTstate(t, 1)

	ts.fetch(ts.srvs[0], []string{PATH}, []string{PATH})

	kid, ok := ts.bins.GetBinKernelID(PROG)
	assert.True(ts.T, ok)

	pn := chunk.ChunkdPath(kid)
	ts.fetch(ts.srvs[0], []string{pn}, []string{pn})

	ts.Shutdown()
}

func TestFetchPath(t *testing.T) {
	ts := newTstate(t, 1)

	// fetch through chunkd1 so that it has it cached
	pn1 := chunk.ChunkdPath(ts.srvs[1])
	ts.fetch(ts.srvs[1], []string{pn1, PATH}, []string{PATH})

	// fetch through chunkd 0 with chunkd1 in search path,
	// so data should come from chunkd1
	ts.fetch(ts.srvs[0], []string{pn1, PATH}, []string{pn1})
	ts.Shutdown()
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
	ts.Shutdown()
}