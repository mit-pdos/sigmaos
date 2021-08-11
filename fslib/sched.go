package fslib

import (
	"path"

	np "ulambda/ninep"
)

const (
	waitq     = "name/waitq"
	spawned   = "name/spawned"
	WAIT_LOCK = "wait-lock."
)

func waitFilePath(pid string) string {
	return path.Join(spawned, waitFileName(pid))
}

func waitFileName(pid string) string {
	return LockName(WAIT_LOCK + pid)
}

// XXX Currently used by gg
func (fl *FsLib) HasBeenSpawned(pid string) bool {
	_, err := fl.Stat(waitFilePath(pid))
	if err == nil {
		return true
	}
	return false
}

// XXX Currently used by kv/monitor.go
func (fl *FsLib) ReadWaitQ() ([]*np.Stat, error) {
	d, err := fl.ReadDir(waitq)
	if err != nil {
		return d, err
	}
	return d, err
}
