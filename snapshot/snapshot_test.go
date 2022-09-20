package snapshot_test

import (
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/leaderclnt"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/semclnt"
	"ulambda/test"
)

const (
	REPLICA_SYMLINK = "name/symlink"
	MUTEX_PATH      = REPLICA_SYMLINK + "/mutex"
)

func spawnMemfs(ts *test.Tstate, pid proc.Tpid) {
	p := proc.MakeProcPid(pid, "user/memfsd", []string{"dummy"})
	err := ts.Spawn(p)
	assert.Nil(ts.T, err, "Spawn")
	err = ts.WaitStart(pid)
	assert.Nil(ts.T, err, "WaitStart")
}

func killMemfs(ts *test.Tstate, pid proc.Tpid) {
	err := ts.Evict(pid)
	assert.Nil(ts.T, err, "Evict")
	status, err := ts.WaitExit(pid)
	assert.True(ts.T, status.IsStatusEvicted(), "Wrong exit status")
}

func takeSnapshot(ts *test.Tstate, fsl *fslib.FsLib, pid proc.Tpid) []byte {
	p := path.Join(np.MEMFS, pid.String(), np.SNAPDEV)
	// Read its snapshot file.
	b, err := fsl.GetFile(p)
	assert.Nil(ts.T, err, "Read Snapshot")
	assert.True(ts.T, len(b) > 0, "Snapshot len")
	return b
}

func restoreSnapshot(ts *test.Tstate, fsl *fslib.FsLib, pid proc.Tpid, b []byte) {
	p := path.Join(np.MEMFS, pid.String(), np.SNAPDEV)
	sz, err := fsl.SetFile(p, b, np.OWRITE, 0)
	assert.Nil(ts.T, err, "Write snapshot")
	assert.Equal(ts.T, sz, np.Tsize(len(b)), "Snapshot write wrong size")
}

func symlinkReplicas(ts *test.Tstate, pids []proc.Tpid) {
	addrs := []string{}
	for _, pid := range pids {
		p := path.Join(np.MEMFS, pid.String())
		b, err := ts.GetFile(p)
		addr := strings.TrimSuffix(string(b), ":pubkey")
		assert.Nil(ts.T, err, "Get addr")
		addrs = append(addrs, addr)
	}
	db.DPrintf(db.ALWAYS, "Replica addrs: %v", addrs)
	err := ts.Symlink(fslib.MakeTarget(addrs), REPLICA_SYMLINK, 0777)
	assert.Nil(ts.T, err, "Symlink")
}

func putFiles(ts *test.Tstate, n int) {
	for i := 0; i < n; i++ {
		i_str := strconv.Itoa(i)
		_, err := ts.PutFile(path.Join(REPLICA_SYMLINK, i_str), 0777, np.ORDWR, []byte(i_str))
		assert.Nil(ts.T, err, "Putfile")
	}
}

func checkFiles(ts *test.Tstate, n int) {
	for i := 0; i < n; i++ {
		i_str := strconv.Itoa(i)
		b, err := ts.GetFile(path.Join(REPLICA_SYMLINK, i_str))
		assert.Nil(ts.T, err, "Getfile:")
		if err != nil {
			db.DPrintf(db.ALWAYS, "File err %v", err.Error())
		}
		assert.Equal(ts.T, i_str, string(b), "File contents")
	}
}

func fenceMemfs(ts *test.Tstate, fsl *fslib.FsLib, pid proc.Tpid) {
	lc := leaderclnt.MakeLeaderClnt(fsl, path.Join(np.MEMFS, pid.String(), "leader"), 0777)
	_, err := lc.AcquireFencedEpoch(nil, []string{path.Join(np.MEMFS, pid.String())})
	assert.Nil(ts.T, err, "acquire")
}

func TestMakeSnapshotSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.MkDir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid := proc.Tpid("replica-a")
	spawnMemfs(ts, pid)

	fsl1 := fslib.MakeFsLib("test-fsl1")
	takeSnapshot(ts, fsl1, pid)

	ts.Shutdown()
}

func TestMakeSnapshotSimpleWithFence(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.MkDir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid := proc.Tpid("replica-a" + proc.GenPid().String())
	spawnMemfs(ts, pid)

	fsl1 := fslib.MakeFsLib("test-fsl1")

	// Fence the memfs
	fenceMemfs(ts, fsl1, pid)

	takeSnapshot(ts, fsl1, pid)

	ts.Shutdown()
}

func TestRestoreSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.MkDir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid := proc.Tpid("replica-a" + proc.GenPid().String())
	spawnMemfs(ts, pid)

	fsl1 := fslib.MakeFsLib("test-fsl1")
	db.DPrintf("TEST", "About to take snapshot")
	b := takeSnapshot(ts, fsl1, pid)
	db.DPrintf("TEST", "Done take snapshot")
	db.DPrintf("TEST", "About to restore snapshot")
	restoreSnapshot(ts, fsl1, pid, b)
	db.DPrintf("TEST", "Done restore snapshot")

	ts.Shutdown()
}

func TestRestoreSimpleWithFence(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.MkDir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid := proc.Tpid("replica-a" + proc.GenPid().String())
	spawnMemfs(ts, pid)

	fsl1 := fslib.MakeFsLib("test-fsl1")

	// Fence the memfs
	fenceMemfs(ts, fsl1, pid)

	b := takeSnapshot(ts, fsl1, pid)
	restoreSnapshot(ts, fsl1, pid, b)

	ts.Shutdown()
}

func TestRestoreStateSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	N_FILES := 100

	err := ts.MkDir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid1 := proc.Tpid("replica-a" + proc.GenPid().String())
	spawnMemfs(ts, pid1)

	// Spawn another one
	pid2 := proc.Tpid("replica-b" + proc.GenPid().String())
	spawnMemfs(ts, pid2)

	symlinkReplicas(ts, []proc.Tpid{pid1, pid2})

	// Create some server-side state in the first replica.
	putFiles(ts, N_FILES)

	// Check the state is there.
	checkFiles(ts, N_FILES)

	fsl1 := fslib.MakeFsLib("test-fsl1")
	_, err = fsl1.Stat(path.Join(np.MEMFS, pid2.String(), np.SNAPDEV) + "/")
	assert.Nil(ts.T, err, "Bad stat: %v", err)

	// Read the snapshot from replica a
	db.DPrintf("TEST", "About to take snapshot")
	b := takeSnapshot(ts, fsl1, pid1)
	db.DPrintf("TEST", "Done take snapshot")

	// Kill the first replica (so future requests hit the second replica).
	killMemfs(ts, pid1)

	db.DPrintf("TEST", "Restoring snapshot")

	// Write the snapshot to replica b
	restoreSnapshot(ts, fsl1, pid2, b)

	db.DPrintf("TEST", "Done restoring snapshot")

	// Check that the files exist on replica b
	checkFiles(ts, N_FILES)

	ts.Shutdown()
}

func TestRestoreBlockingOpSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.MkDir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid1 := proc.Tpid("replica-a" + proc.GenPid().String())
	spawnMemfs(ts, pid1)

	// Spawn another one
	pid2 := proc.Tpid("replica-b" + proc.GenPid().String())
	spawnMemfs(ts, pid2)

	symlinkReplicas(ts, []proc.Tpid{pid1, pid2})

	sem1 := semclnt.MakeSemClnt(ts.FsLib, MUTEX_PATH)
	sem1.Init(0777)

	fsl2 := fslib.MakeFsLib("blocking-cli-2")
	sem2 := semclnt.MakeSemClnt(fsl2, MUTEX_PATH)
	done := make(chan bool)

	go func() {
		err := sem2.Down()
		assert.Nil(ts.T, err, "Sem down")
		done <- true
	}()

	// Make sure to wait long enough for the other client to block server-side.
	time.Sleep(1 * time.Second)

	fsl1 := fslib.MakeFsLib("test-fsl1")
	// Read the snapshot from replica a
	b := takeSnapshot(ts, fsl1, pid1)

	// Write the snapshot to replica b
	restoreSnapshot(ts, fsl1, pid2, b)

	// Kill the first replica (so pending requests hit the second replica).
	killMemfs(ts, pid1)

	// Wait long enough for the second client's blocking request to hit the
	// second replica.
	time.Sleep(1 * time.Second)

	// Ensure the client hasn't unblocked yet.
	ok := true
	select {
	case <-done:
		ok = false
	default:
	}

	assert.True(ts.T, ok, "Didn't wait at second server")

	err1 := sem1.Up()
	assert.Nil(ts.T, err1, "Sem up")

	ok = <-done

	assert.True(ts.T, ok, "Didn't release from second server")

	ts.Shutdown()
}
