package chunksrv_test

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

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
	paths  []string
}

func newTstate(t *testing.T, n int) *Tstate {
	ts := &Tstate{paths: []string{PATH}}
	s, err := test.NewTstateAll(t)
	assert.Nil(t, err)
	ts.Tstate = s

	ts.ckclnt = chunkclnt.NewChunkClnt(s.FsLib)
	ts.ckclnt.UpdateChunkds()

	err = s.BootNode(n)
	assert.Nil(t, err, "Boot node: %v", err)
	db.DPrintf(db.TEST, "Done boot node %d", n)

	ckclnt := chunkclnt.NewChunkClnt(ts.FsLib)
	ckclnt.UpdateChunkds()
	srvs, err := ckclnt.GetSrvs()
	assert.Nil(t, err)

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
	assert.Equal(ts.T, st.Length, uint64(fi.Size()))
}

func TestFetchOne(t *testing.T) {
	ts := newTstate(t, 0)
	pid := ts.ProcEnv().GetPID()

	srv, err := ts.ckclnt.RandomSrv()
	assert.Nil(t, err)

	st, err := ts.ckclnt.GetFileStat(srv, PROG, pid, sp.ROOTREALM, ts.paths)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "st %v\n", st)

	err = ts.ckclnt.FetchBinary(srv, PROG, pid, sp.ROOTREALM, sp.Tsize(st.Length), ts.paths)
	assert.Nil(t, err, "err %v", err)

	ts.check(srv, st)

	ts.Shutdown()
}

func TestFetchMulti(t *testing.T) {
	ts := newTstate(t, 2)

	ts.Shutdown()
}
