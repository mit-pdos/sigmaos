package leadertest

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/test"
)

func runLeaders(t *testing.T, ts *test.Tstate, sec string) (string, []string) {
	const (
		N = 2
	)
	pids := []string{}

	// XXX use the same dir independent of machine running proc
	ts.RmDir(np.UX + "/" + np.FENCEDIR)

	dir := np.UX + "/~ip/outdir/"
	fn := dir + "out"
	ts.RmDir(dir)
	err := ts.MkDir(dir, 0777)
	_, err = ts.PutFile(fn, 0777, np.OWRITE, []byte{})
	assert.Nil(t, err, "putfile")

	for i := 0; i < N; i++ {
		last := ""
		if i == N-1 {
			last = "last"
		}
		p := proc.MakeProc("bin/user/leadertest-leader", []string{dir, last, sec})
		err = ts.Spawn(p)
		assert.Nil(t, err, "Spawn")

		err = ts.WaitStart(p.Pid)
		assert.Nil(t, err, "WaitStarted")

		pids = append(pids, p.Pid)
	}

	time.Sleep(1000 * time.Millisecond)

	// wait for last one; the other procs cannot communicate exit
	// status to test because test's procdir is in name/
	_, err = ts.WaitExit(pids[len(pids)-1])
	assert.Nil(t, err, "WaitExit")

	return fn, pids
}

func check(t *testing.T, ts *test.Tstate, fn string, pids []string) {
	rdr, err := ts.OpenReader(fn)
	assert.Nil(t, err, "GetFile")
	m := make(map[string]bool)
	last := ""
	err = rdr.ReadJsonStream(func() interface{} { return new(Config) }, func(a interface{}) error {
		conf := *a.(*Config)
		log.Printf("conf: %v\n", conf)
		if last != conf.Leader {
			assert.Equal(t, conf.Pid, conf.Leader, "new leader")
			_, ok := m[conf.Leader]
			assert.False(t, ok, "pid")
			m[conf.Leader] = true
			last = conf.Leader
		}
		return nil
	})
	assert.Nil(t, err, "StreamJson")
	for _, pid := range pids {
		assert.True(t, m[pid], "pids")
	}
}

func TestOldPrimary(t *testing.T) {
	ts := test.MakeTstateAll(t)
	fn, pids := runLeaders(t, ts, "")
	check(t, ts, fn, pids)

	log.Printf("exit\n")

	ts.Shutdown()
}

func TestOldProc(t *testing.T) {
	ts := test.MakeTstateAll(t)
	fn, pids := runLeaders(t, ts, "child")
	check(t, ts, fn, pids)

	log.Printf("exit\n")

	ts.Shutdown()
}
