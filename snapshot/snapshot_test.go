package snapshot_test

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/test"
)

func spawnMemfs(ts *test.Tstate, pid string) {
	p := proc.MakeProcPid(pid, "bin/user/memfsd", []string{"dummy"})
	err := ts.Spawn(p)
	assert.Nil(ts.T, err, "Spawn")

	err = ts.WaitStart(pid)
	assert.Nil(ts.T, err, "WaitStart")
}

func TestTakeSnapshotSimple(t *testing.T) {
	ts := test.MakeTstateAll(t)

	err := ts.Mkdir(np.MEMFS, 0777)
	assert.Nil(t, err, "Mkdir")

	// Spawn a dummy-replicated memfs
	pid := "12345"
	spawnMemfs(ts, pid)

	// Read its snapshot file.
	b, err := ts.GetFile(path.Join(np.MEMFS, pid, "snapshot"))
	assert.Nil(t, err, "Snapshot")

	assert.True(t, len(b) > 0, "Snapshot len")

	ts.Shutdown()
}
