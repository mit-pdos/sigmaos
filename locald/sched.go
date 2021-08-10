package locald

import (
	"log"
	"path"

	"ulambda/fslib"
	np "ulambda/ninep"
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

// Claim a job by moving it from the runq to the claimed dir
func (ld *LocalD) ClaimRunQJob(queuePath string, pid string) ([]byte, bool) {
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
