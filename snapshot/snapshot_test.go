package snapshot_test

import (
	"testing"
	"time"

	"ulambda/test"
)

func TestSnapshotSimple(t *testing.T) {
	ts := test.MakeTstate(t)
	time.Sleep(30 * time.Second)
	ts.Shutdown()
}
