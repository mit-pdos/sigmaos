package procd

import (
	"log"
	"path"

	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
)

func (pd *Procd) WaitForJob() error {
	// Wait for something runnable
	return pd.LockFile(fslib.LOCKS, fslib.JOB_SIGNAL)
}

func (pd *Procd) ReadJob(dir string, job string) ([]byte, error) {
	b, _, err := pd.GetFile(path.Join(dir, job))
	return b, err
}

func (pd *Procd) ReadRunQ(dir string) ([]*np.Stat, error) {
	d, err := pd.ReadDir(dir)
	if err != nil {
		return d, err
	}
	return d, err
}

// Claim a job by moving it from the runq to the claimed dir
func (pd *Procd) ClaimRunQJob(queuePath string, pid string) ([]byte, bool) {
	// Write the file to reset its mtime (to avoid racing with Monitor). Ignore
	// errors in the event we lose the race.
	pd.WriteFile(path.Join(queuePath, pid), []byte{})
	err := pd.Rename(path.Join(queuePath, pid), path.Join(proc.CLAIMED, pid))
	if err != nil {
		return []byte{}, false
	}
	// Create an ephemeral file to mark that procd hasn't crashed
	err = pd.MakeFile(path.Join(proc.CLAIMED_EPH, pid), 0777|np.DMTMP, np.OWRITE, []byte{})
	if err != nil {
		log.Printf("Error making ephemeral claimed job file: %v", err)
	}
	b, _, err := pd.GetFile(path.Join(proc.CLAIMED, pid))
	if err != nil {
		log.Printf("Error reading claimed job: %v", err)
		return []byte{}, false
	}
	// We shouldn't hold the "new job" lock while running a lambda/doing work
	pd.SignalNewJob()
	return b, true
}
