package tx_test

import (
	"log"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/realm"
	"ulambda/tx"
)

const (
	TX_LOCK_DIR  = "name/tx-lock-dir"
	TX_STATE_DIR = "name/tx-state-dir"
)

type Tstate struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	t   *testing.T
	e   *realm.TestEnv
	cfg *realm.RealmConfig
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	bin := ".."
	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg

	db.Name("tx_test")
	ts.FsLib = fslib.MakeFsLibAddr("txtest", cfg.NamedAddrs)
	ts.t = t

	ts.ProcClnt = procclnt.MakeProcClntInit(proc.GenPid(), ts.FsLib, cfg.NamedAddrs)

	return ts
}

func runMemfs(ts *Tstate) string {
	t := proc.MakeProc("user/memfsd", []string{""})
	err := ts.Spawn(t)
	assert.Nil(ts.t, err, "start memfs")
	return t.Pid
}

func setupEnv(ts *Tstate) string {
	// Set up memfs state
	err := ts.MkDir(named.MEMFS, 0777)
	assert.Nil(ts.t, err, "mkdir memfs")
	pid1 := runMemfs(ts)

	err = ts.MakeFile(path.Join(named.MEMFS, pid1, "x"), 0777, np.OWRITE, []byte("100"))
	assert.Nil(ts.t, err, "MakeFile 1")

	err = ts.MakeFile(path.Join(named.MEMFS, pid1, "y"), 0777, np.OWRITE, []byte("100"))
	assert.Nil(ts.t, err, "MakeFile 2")

	// Set up tx state
	err = ts.MkDir(TX_LOCK_DIR, 0777)
	assert.Nil(ts.t, err, "mkdir txlock")

	err = ts.MkDir(TX_STATE_DIR, 0777)
	assert.Nil(ts.t, err, "mkdir txstate")

	return pid1
}

func TestSingleTx(t *testing.T) {
	ts := makeTstate(t)

	pid1 := setupEnv(ts)

	id1 := "tx1"
	t1 := tx.MakeTx(ts.FsLib, id1, TX_STATE_DIR, TX_LOCK_DIR)

	err := t1.Begin()
	assert.Nil(ts.t, err, "T1 Begin")

	b, err := t1.ReadFile(path.Join(named.MEMFS, pid1, "x"))
	assert.Nil(ts.t, err, "T1 RF 1")
	assert.Equal(ts.t, string(b), "100", "T1 RF 1 Not expected")
	log.Printf("Abcd")

	ts.e.Shutdown()
}
