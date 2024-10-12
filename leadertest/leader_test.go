package leadertest

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	DIR = sp.NAMED + "outdir"
)

func runLeaders(t *testing.T, ts *test.Tstate, sec string) (string, []sp.Tpid) {
	const (
		N = 10
	)
	pids := []sp.Tpid{}

	ts.RmDir(DIR)
	fn := filepath.Join(DIR, OUT)

	ts.Remove(LEADERFN)
	err := ts.MkDir(DIR, 0777)
	_, err = ts.PutFile(fn, 0777, sp.OWRITE, []byte{})
	assert.Nil(t, err, "putfile")

	for i := 0; i < N; i++ {
		last := ""
		if i == N-1 {
			last = "last"
		}
		p := proc.NewProc("leadertest-leader", []string{DIR, last, sec})
		err = ts.Spawn(p)
		assert.Nil(t, err, "Spawn")

		err = ts.WaitStart(p.GetPid())
		assert.Nil(t, err, "WaitStarted")

		pids = append(pids, p.GetPid())
	}

	for _, pid := range pids {
		_, err = ts.WaitExit(pid)
		if pid == pids[len(pids)-1] {
			assert.Nil(t, err, "WaitExit")
		}
	}
	return fn, pids
}

func check(t *testing.T, ts *test.Tstate, fn string, pids []sp.Tpid) {
	rdr, err := ts.OpenReader(fn)
	assert.Nil(t, err, "GetFile")
	m := make(map[sp.Tpid]bool)
	last := sp.NO_PID
	e := sp.Tepoch(0)
	err = fslib.JsonReader(rdr, func() interface{} { return new(Config) }, func(a interface{}) error {
		conf := *a.(*Config)
		db.DPrintf(db.ALWAYS, "conf: %v", conf)
		if conf.Leader == sp.NO_PID && e != 0 {
			assert.Equal(t, conf.Epoch, e)
		} else if last != conf.Leader { // new leader
			assert.Equal(t, conf.Pid, conf.Leader, "new leader")
			_, ok := m[conf.Leader]
			assert.False(t, ok, "pid")
			m[conf.Leader] = true
			last = conf.Leader
			assert.True(t, conf.Epoch > e, "Conf epoch %v <= %v", conf.Epoch, e)
			e = conf.Epoch
		}
		return nil
	})
	assert.Nil(t, err, "StreamJson")
	for _, pid := range pids {
		assert.True(t, m[pid], "pids")
	}
}

func TestCompile(t *testing.T) {
}

func TestOldPrimary(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	fn, pids := runLeaders(t, ts, "")
	check(t, ts, fn, pids)
	ts.Shutdown()
}

func TestOldProc(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	fn, pids := runLeaders(t, ts, "child")
	check(t, ts, fn, pids)
	ts.Shutdown()
}
