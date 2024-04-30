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

func TestFetch(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err)

	pid := ts.ProcEnv().GetPID()

	ckclnt := chunkclnt.NewChunkClnt(ts.FsLib)
	ckclnt.UpdateChunkds()

	srv, err := ckclnt.RandomSrv()
	assert.Nil(t, err)

	pn := chunksrv.PathHostKernelRealm(srv, sp.ROOTREALM)
	os.Mkdir(pn, 0700)

	st, err := ckclnt.GetFileStat(srv, PROG, pid, sp.ROOTREALM, []string{PATH})
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "st %v\n", st)

	err = ckclnt.FetchBinary(srv, PROG, pid, sp.ROOTREALM, sp.Tsize(st.Length), []string{PATH})
	assert.Nil(t, err, "err %v", err)

	pn = path.Join(pn, PROG)
	fi, err := os.Stat(pn)
	assert.Nil(t, err)
	assert.Equal(t, st.Length, uint64(fi.Size()))

	ts.Shutdown()
}
