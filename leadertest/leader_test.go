package leadertest

import (
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/crash"
	"ulambda/delay"
	"ulambda/fslib"
	"ulambda/leaderclnt"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/test"
)

// Test if a primary cannot write to a fenced server after primary
// fails
func TestOldPrimaryOnce(t *testing.T) {
	ts := test.MakeTstateAll(t)
	leadername := "name/l"

	dirux := np.UX + "/~ip/outdir"
	ts.MkDir(dirux, 0777)
	ts.Remove(dirux + "/f")

	fsldl := fslib.MakeFsLibAddr("leader", fslib.Named())

	ch := make(chan bool)
	go func() {
		leader := leaderclnt.MakeLeaderClnt(fsldl, leadername, 0)
		err := leader.AcquireLeadership([]byte{})
		assert.Nil(t, err, "AcquireLeadership")

		fd, err := fsldl.Create(dirux+"/f", 0777, np.OWRITE)
		assert.Nil(t, err, "Create")

		ch <- true

		log.Printf("partition from named..\n")

		crash.Partition(fsldl)
		delay.Delay(10)

		// fsldl lost primary status, and ts should have it by
		// now so this write to ux server should fail
		_, err = fsldl.Write(fd, []byte(strconv.Itoa(1)))
		assert.NotNil(t, err, "Write")

		fsldl.Close(fd)

		ch <- true
	}()

	// Wait until other thread is primary
	<-ch

	// When other thread partitions, we become primary and install
	// fence.
	leader := leaderclnt.MakeLeaderClnt(ts.FsLib, leadername, 0)
	err := leader.AcquireLeadership([]byte{})
	assert.Nil(t, err, "AcquireLeadership")

	<-ch

	fd, err := ts.Open(dirux+"/f", np.OREAD)
	assert.Nil(t, err, "Open")
	b, err := ts.Read(fd, 100)
	assert.Equal(ts.T, 0, len(b))

	ts.Shutdown()
}

func runPrimaries(t *testing.T, ts *test.Tstate, sec string) (string, []string) {
	const (
		N = 10
	)
	pids := []string{}

	// XXX use the same dir independent of machine running proc
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
		p := proc.MakeProc("bin/user/leadertest-leader", []string{"name/fence", dir, last, sec})
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

func checkPrimaries(t *testing.T, ts *test.Tstate, fn string, pids []string) {
	rdr, err := ts.OpenReader(fn)
	assert.Nil(t, err, "GetFile")
	m := make(map[string]bool)
	err = rdr.ReadJsonStream(func() interface{} { return new(string) }, func(a interface{}) error {
		pid := *a.(*string)
		log.Printf("pid: %v\n", pid)
		_, ok := m[pid]
		assert.False(t, ok, "pid")
		m[pid] = true
		return nil
	})
	assert.Nil(t, err, "StreamJson")
	for _, pid := range pids {
		assert.True(t, m[pid], "pids")
	}
}

func TestOldPrimaryConcur(t *testing.T) {
	ts := test.MakeTstateAll(t)
	fn, pids := runPrimaries(t, ts, "")
	checkPrimaries(t, ts, fn, pids)

	log.Printf("exit\n")

	ts.Shutdown()
}
