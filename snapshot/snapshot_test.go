package snapshot_test

import (
	"log"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/test"
)

const (
	REPLICA_SYMLINK = "name/symlink"
)

func spawnMemfs(ts *test.Tstate, pid string) {
	p := proc.MakeProcPid(pid, "bin/user/memfsd", []string{"dummy"})
	err := ts.Spawn(p)
	assert.Nil(ts.T, err, "Spawn")
	err = ts.WaitStart(pid)
	assert.Nil(ts.T, err, "WaitStart")
}

func killMemfs(ts *test.Tstate, pid string) {
	err := ts.Evict(pid)
	assert.Nil(ts.T, err, "Evict")
	status, err := ts.WaitExit(pid)
	assert.True(ts.T, status.IsStatusEvicted(), "Wrong exit status")
}

func takeSnapshot(ts *test.Tstate, pid string) []byte {
	p := path.Join(np.MEMFS, pid, "snapshot")
	// Read its snapshot file.
	b, err := ts.GetFile(p)
	assert.Nil(ts.T, err, "Read Snapshot")
	assert.True(ts.T, len(b) > 0, "Snapshot len")
	return b
}

func restoreSnapshot(ts *test.Tstate, pid string, b []byte) {
	p := path.Join(np.MEMFS, pid, "snapshot")
	// Restore needs to happen from a fresh fslib, otherwise state like fids may
	// be missing during future walks.
	fsl := fslib.MakeFsLib("snapshot-restore")
	sz, err := fsl.SetFile(p, b, 0)
	assert.Nil(ts.T, err, "Write snapshot")
	assert.Equal(ts.T, sz, np.Tsize(len(b)), "Snapshot write wrong size")
}

func symlinkReplicas(ts *test.Tstate, pids []string) {
	addrs := []string{}
	for _, pid := range pids {
		p := path.Join(np.MEMFS, pid)
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
		_, err := ts.PutFile(path.Join(REPLICA_SYMLINK, i_str), 0777, np.OREAD|np.OWRITE, []byte(i_str))
		assert.Nil(ts.T, err, "Putfile")
	}
}

func checkFiles(ts *test.Tstate, basePath string, n int) {
	for i := 0; i < n; i++ {
		i_str := strconv.Itoa(i)
		b, err := ts.GetFile(path.Join(basePath, i_str))
		assert.Nil(ts.T, err, "Getfile:")
		if err != nil {
			log.Printf(err.Error())
		}
		assert.Equal(ts.T, i_str, string(b), "File contents")
	}
}

func TestMakeSnapshotSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.Mkdir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid := "replica-a"
	spawnMemfs(ts, pid)

	takeSnapshot(ts, pid)

	ts.Shutdown()
}

func TestRestoreSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.Mkdir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid := "replica-a"
	spawnMemfs(ts, pid)

	b := takeSnapshot(ts, pid)
	restoreSnapshot(ts, pid, b)

	ts.Shutdown()
}

func TestRestoreStateSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	N_FILES := 1

	err := ts.Mkdir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid1 := "replica-a"
	spawnMemfs(ts, pid1)

	// Spawn another one
	pid2 := "replica-b"
	spawnMemfs(ts, pid2)

	symlinkReplicas(ts, []string{pid1, pid2})

	// Create some server-side state in the first replica.
	putFiles(ts, N_FILES)

	// Check the state is there.
	checkFiles(ts, path.Join(np.MEMFS, pid1), N_FILES)

	// Read the snapshot from replica a
	b := takeSnapshot(ts, pid1)

	// Kill the first replica (so future requests hit the second replica).
	killMemfs(ts, pid1)

	// Write the snapshot to replica b
	restoreSnapshot(ts, pid2, b)

	// Check that the files exist on replica b
	checkFiles(ts, path.Join(np.MEMFS, pid2), N_FILES)

	ts.Shutdown()
}
