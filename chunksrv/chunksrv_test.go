package chunksrv_test

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/chunkclnt"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	PROG  = "sleeper"
	PATH  = "name/ux/~local/bin/user/common/"
	PID   = sp.Tpid("xxx")
	CACHE = "/tmp/sigmaos-bin/"
)

func TestFetch(t *testing.T) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err)

	ckclnt := chunkclnt.NewChunkClnt(ts.FsLib)
	ckclnt.UpdateChunkds()

	srv, err := ckclnt.RandomSrv()
	assert.Nil(t, err)

	pn := path.Join(CACHE, srv, sp.ROOTREALM.String())
	os.Mkdir(pn, 0700)

	st, err := ckclnt.GetFileStat(srv, PROG, PID, sp.ROOTREALM, []string{PATH})
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "st %v\n", st)

	err = ckclnt.FetchBinary(srv, PROG, PID, sp.ROOTREALM, sp.Tsize(st.Length), []string{PATH})
	assert.Nil(t, err, "err %v", err)

	pn = path.Join(pn, PROG)
	fi, err := os.Stat(pn)
	assert.Nil(t, err)
	assert.Equal(t, st.Length, uint64(fi.Size()))

	ts.Shutdown()
}
