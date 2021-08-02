package fslib

import (
	"encoding/json"
	"log"
	"path"
	"time"

	np "ulambda/ninep"
)

const (
	RUNQ          = "name/runq"
	RUNQLC        = "name/runqlc"
	WAITQ         = "name/waitq"
	CLAIMED       = "name/claimed"
	CLAIMED_EPH   = "name/claimed_ephemeral"
	SPAWNED       = "name/spawned"
	RET_STAT      = "name/retstat"
	JOB_SIGNAL    = "job-signal"
	WAIT_LOCK     = "wait-lock."
	CRASH_TIMEOUT = 1
	// XXX REMOVE
	WAITFILE_PADDING = 1000
)

// Notify localds that a job has become runnable
func (fl *FsLib) SignalNewJob() error {
	// Needs to be done twice, since someone waiting on the signal will create a
	// new lock file, even if they've crashed
	return fl.UnlockFile(LOCKS, JOB_SIGNAL)
}

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

// Check if a job crashed. We know this is the case if it is fslib.CLAIMED, but
// the corresponding fslib.CLAIMED_EPH file is missing (locald crashed). Since, upon
// successful ClaimJob, there is a very short window during with the fslib.CLAIMED
// file exists but the fslib.CLAIMED_EPH file does not exist, wait a short amount of
// time (CRASH_TIMEOUT) before declaring a job as failed.
func (fl *FsLib) JobCrashed(pid string) bool {
	_, err := fl.Stat(path.Join(CLAIMED_EPH, pid))
	if err != nil {
		stat, err := fl.Stat(path.Join(CLAIMED, pid))
		// If it has fully exited (both claimed & claimed_ephemeral are gone)
		if err != nil {
			return false
		}
		// If it is in the process of being claimed
		if int64(stat.Mtime+CRASH_TIMEOUT) < time.Now().Unix() {
			return true
		}
	}
	return false
}

// Move a job from fslib.CLAIMED to fslib.RUNQ
func (fl *FsLib) RestartJob(pid string) error {
	// XXX read fslib.CLAIMED to find out if it is LC?
	b, _, err := fl.GetFile(path.Join(CLAIMED, pid))
	if err != nil {
		return nil
	}
	var attr Attr
	err = json.Unmarshal(b, &attr)
	if err != nil {
		log.Printf("Error unmarshalling in RestartJob: %v", err)
	}
	runq := RUNQ
	if attr.Type == T_LC {
		runq = RUNQLC
	}
	fl.Rename(path.Join(CLAIMED, pid), path.Join(runq, pid))
	// Notify localds that a job has become runnable
	fl.SignalNewJob()
	return nil
}
