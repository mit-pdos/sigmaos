package procd

import (
	"log"
	"path"

	np "ulambda/ninep"
	"ulambda/proc"
)

func (pd *Procd) WaitForJob() {
	// Wait for something runnable
	pd.jobLock.Lock()
}

func (pd *Procd) ReadRunQ(dir string) ([]*np.Stat, error) {
	d, err := pd.ReadDir(dir)
	if err != nil {
		return d, err
	}
	return d, err
}

// Claim a job by moving it from the runq to the claimed dir
func (pd *Procd) ClaimRunQJob(queuePath string, pid string) bool {
	// Write the file to reset its mtime (to avoid racing with Monitor). Ignore
	// errors in the event we lose the race.
	pd.WriteFile(path.Join(queuePath, pid), []byte{})
	err := pd.Rename(path.Join(queuePath, pid), path.Join(proc.CLAIMED, pid))
	if err != nil {
		return false
	}
	// Create an ephemeral file to mark that procd hasn't crashed
	err = pd.MakeFile(path.Join(proc.CLAIMED_EPH, pid), 0777|np.DMTMP, np.OWRITE, []byte{})
	if err != nil {
		log.Printf("Error making ephemeral claimed job file: %v", err)
	}
	_, _, err = pd.GetFile(path.Join(proc.CLAIMED, pid))
	if err != nil {
		log.Printf("Error reading claimed job: %v", err)
		return false
	}
	// We shouldn't hold the "new job" lock while running a lambda/doing work
	pd.SignalNewJob()
	return true
}
