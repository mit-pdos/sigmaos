package locald

import (
	"encoding/json"
	"log"
	"path"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
)

func (ld *LocalD) WaitForJob() error {
	// Wait for something runnable
	return ld.LockFile(fslib.LOCKS, fslib.JOB_SIGNAL)
}

func (ld *LocalD) ReadJob(dir string, job string) ([]byte, error) {
	b, _, err := ld.GetFile(path.Join(dir, job))
	return b, err
}

func (ld *LocalD) ReadRunQ(dir string) ([]*np.Stat, error) {
	d, err := ld.ReadDir(dir)
	if err != nil {
		return d, err
	}
	return d, err
}

func (ld *LocalD) MarkJobRunnable(pid string, t proc.Ttype) error {
	if t == proc.T_LC {
		ld.Rename(path.Join(fslib.WAITQ, pid), path.Join(fslib.RUNQLC, pid))
	} else {
		ld.Rename(path.Join(fslib.WAITQ, pid), path.Join(fslib.RUNQ, pid))
	}
	// Notify localds that a job has become runnable
	ld.SignalNewJob()
	return nil
}

// Return true if the job has started. This function assumes the job has at
// least been spawned, and hasn't exited yet. If the job file is not present,
// we assume that it has already started (and probably exited).
func (ld *LocalD) JobStarted(pid string) bool {
	ld.LockFile(fslib.LOCKS, proc.WaitFilePath(pid))
	defer ld.UnlockFile(fslib.LOCKS, proc.WaitFilePath(pid))

	// Get the current contents of the file & its version
	b, _, err := ld.GetFile(proc.WaitFilePath(pid))
	if err != nil {
		db.DLPrintf("LOCALD", "Job file not found JobStarted: %v, %v", proc.WaitFilePath(pid), err)
		return true
	}
	var wf fslib.WaitFile
	err = json.Unmarshal(b, &wf)
	if err != nil {
		log.Fatalf("Error unmarshalling waitfile: %v, %v", string(b), err)
	}
	return wf.Started
}

// Claim a job by moving it from the runq to the claimed dir
func (ld *LocalD) ClaimRunQJob(dir string, pid string) ([]byte, bool) {
	return ld.claimJob(dir, pid)
}

// Claim a job by moving it from the runq to the claimed dir
func (ld *LocalD) ClaimWaitQJob(pid string) ([]byte, bool) {
	return ld.claimJob(fslib.WAITQ, pid)
}

func (ld *LocalD) claimJob(queuePath string, pid string) ([]byte, bool) {
	// Write the file to reset its mtime (to avoid racing with Monitor). Ignore
	// errors in the event we lose the race.
	ld.WriteFile(path.Join(queuePath, pid), []byte{})
	err := ld.Rename(path.Join(queuePath, pid), path.Join(fslib.CLAIMED, pid))
	if err != nil {
		return []byte{}, false
	}
	// Create an ephemeral file to mark that locald hasn't crashed
	err = ld.MakeFile(path.Join(fslib.CLAIMED_EPH, pid), 0777|np.DMTMP, np.OWRITE, []byte{})
	if err != nil {
		log.Printf("Error making ephemeral claimed job file: %v", err)
	}
	b, _, err := ld.GetFile(path.Join(fslib.CLAIMED, pid))
	if err != nil {
		log.Printf("Error reading claimed job: %v", err)
		return []byte{}, false
	}
	// We shouldn't hold the "new job" lock while running a lambda/doing work
	ld.SignalNewJob()
	return b, true
}
