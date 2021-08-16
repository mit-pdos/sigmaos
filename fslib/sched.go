package fslib

import (
	"path"
)

const (
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
