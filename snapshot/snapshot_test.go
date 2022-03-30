package snapshot_test

import (
	"log"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
	p := proc.MakeProcPid(pid, "bin/user/memfsd", []string{"dummy"})
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

func takeSnapshot(ts *test.Tstate, pid proc.Tpid) []byte {
	p := path.Join(np.MEMFS, pid.String(), np.SNAPDEV)
	// Read its snapshot file.
	b, err := ts.GetFile(p)
	assert.Nil(ts.T, err, "Read Snapshot")
	assert.True(ts.T, len(b) > 0, "Snapshot len")
	return b
}

func restoreSnapshot(ts *test.Tstate, pid proc.Tpid, b []byte) {
	p := path.Join(np.MEMFS, pid.String(), np.SNAPDEV)
	// Restore needs to happen from a fresh fslib, otherwise state like fids may
	// be missing during future walks.
	fsl := fslib.MakeFsLib("snapshot-restore")
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
	log.Printf("Replica addrs: %v", addrs)
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
			log.Printf(err.Error())
		}
		assert.Equal(ts.T, i_str, string(b), "File contents")
	}
}

func fenceMemfs(ts *test.Tstate, pid proc.Tpid) {
	lc := leaderclnt.MakeLeaderClnt(ts.FsLib, path.Join(np.MEMFS, pid.String(), "leader"), 0777)
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

	takeSnapshot(ts, pid)

	ts.Shutdown()
}

func TestMakeSnapshotSimpleWithFence(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.MkDir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid := proc.Tpid("replica-a")
	spawnMemfs(ts, pid)

	// Fence the memfs
	fenceMemfs(ts, pid)

	takeSnapshot(ts, pid)

	ts.Shutdown()
}

func TestRestoreSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.MkDir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid := proc.Tpid("replica-a")
	spawnMemfs(ts, pid)

	b := takeSnapshot(ts, pid)
	restoreSnapshot(ts, pid, b)

	ts.Shutdown()
}

func TestRestoreSimpleWithFence(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.MkDir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid := proc.Tpid("replica-a")
	spawnMemfs(ts, pid)

	// Fence the memfs
	fenceMemfs(ts, pid)

	b := takeSnapshot(ts, pid)
	restoreSnapshot(ts, pid, b)

	ts.Shutdown()
}

func TestRestoreStateSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	N_FILES := 100

	err := ts.MkDir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid1 := proc.Tpid("replica-a")
	spawnMemfs(ts, pid1)

	// Spawn another one
	pid2 := proc.Tpid("replica-b")
	spawnMemfs(ts, pid2)

	symlinkReplicas(ts, []proc.Tpid{pid1, pid2})

	// Create some server-side state in the first replica.
	putFiles(ts, N_FILES)

	// Check the state is there.
	checkFiles(ts, N_FILES)

	// Read the snapshot from replica a
	b := takeSnapshot(ts, pid1)

	// Kill the first replica (so future requests hit the second replica).
	killMemfs(ts, pid1)

	// Write the snapshot to replica b
	restoreSnapshot(ts, pid2, b)

	// Check that the files exist on replica b
	checkFiles(ts, N_FILES)

	ts.Shutdown()
}

func TestRestoreBlockingOpSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.MkDir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid1 := proc.Tpid("replica-a")
	spawnMemfs(ts, pid1)

	// Spawn another one
	pid2 := proc.Tpid("replica-b")
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

	// Read the snapshot from replica a
	b := takeSnapshot(ts, pid1)

	// Write the snapshot to replica b
	restoreSnapshot(ts, pid2, b)

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
