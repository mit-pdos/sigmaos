package fslib

import (
	"path"

	np "ulambda/ninep"
)

const (
	RUNQ        = "name/runq"
	RUNQLC      = "name/runqlc"
	WAITQ       = "name/waitq"
	CLAIMED     = "name/claimed"
	CLAIMED_EPH = "name/claimed_ephemeral"
	SPAWNED     = "name/spawned"
	RET_STAT    = "name/retstat"
	JOB_SIGNAL  = "job-signal"
	WAIT_LOCK   = "wait-lock."
	// XXX REMOVE
	WAITFILE_PADDING = 1000
)

func waitFilePath(pid string) string {
	return path.Join(SPAWNED, waitFileName(pid))
}

func waitFileName(pid string) string {
	return LockName(WAIT_LOCK + pid)
}

func (fl *FsLib) HasBeenSpawned(pid string) bool {
	_, err := fl.Stat(waitFilePath(pid))
	if err == nil {
		return true
	}
	return false
}

func (fl *FsLib) ReadWaitQ() ([]*np.Stat, error) {
	d, err := fl.ReadDir(WAITQ)
	if err != nil {
		return d, err
	}
	return d, err
}

func (fl *FsLib) ReadWaitQJob(pid string) ([]byte, error) {
	b, _, err := fl.GetFile(path.Join(WAITQ, pid))
	return b, err
}

func (fl *FsLib) ReadClaimed() ([]*np.Stat, error) {
	d, err := fl.ReadDir(CLAIMED)
	if err != nil {
		return d, err
	}
	return d, err
}
