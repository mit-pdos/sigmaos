package locald

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"github.com/thanhpk/randstr"

	db "ulambda/debug"
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

func (ld *LocalD) MarkJobRunnable(pid string, t fslib.Ttype) error {
	if t == fslib.T_LC {
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
	ld.LockFile(fslib.LOCKS, waitFilePath(pid))
	defer ld.UnlockFile(fslib.LOCKS, waitFilePath(pid))

	// Get the current contents of the file & its version
	b, _, err := ld.GetFile(waitFilePath(pid))
	if err != nil {
		db.DLPrintf("LOCALD", "Job file not found JobStarted: %v, %v", waitFilePath(pid), err)
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

func (ld *LocalD) UpdatePDeps(pid string) ([]fslib.PDep, error) {
	ld.LockFile(fslib.LOCKS, path.Join(fslib.WAITQ, pid))
	defer ld.UnlockFile(fslib.LOCKS, path.Join(fslib.WAITQ, pid))

	newDeps := []fslib.PDep{}

	b, _, err := ld.GetFile(path.Join(fslib.WAITQ, pid))
	if err != nil {
		return newDeps, err
	}
	var a fslib.Attr
	err = json.Unmarshal(b, &a)
	if err != nil {
		log.Printf("Couldn't unmarshal job in updatefslib.PDeps %v: %v", string(b), err)
	}

	for _, dep := range a.PairDep {
		if dep.Consumer == pid {
			if started := ld.JobStarted(dep.Producer); !started {
				newDeps = append(newDeps, dep)
			}
		}
	}

	// Write back updated deps if
	if len(newDeps) != len(a.PairDep) {
		a.PairDep = newDeps
		b2, err := json.Marshal(a)
		if err != nil {
			log.Fatalf("Error marshalling new pairdeps: %v", err)
		}
		_, err = ld.SetFile(waitFilePath(pid), b2, np.NoV)
		if err != nil {
			log.Printf("Error writing Updatefslib.PDeps: %v, %v", waitFilePath(pid), err)
		}
	}

	return newDeps, nil
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

func (ld *LocalD) makeWaitFile(pid string) error {
	fpath := waitFilePath(pid)
	var wf fslib.WaitFile
	wf.Started = false
	b, err := json.Marshal(wf)
	if err != nil {
		log.Printf("Error marshalling waitfile: %v", err)
	}
	// XXX hack around lack of OTRUNC
	for i := 0; i < fslib.WAITFILE_PADDING; i++ {
		b = append(b, ' ')
	}
	// Make a writable, versioned file
	err = ld.MakeFile(fpath, 0777, np.OWRITE, b)
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() != "EOF" {
		return fmt.Errorf("Error on MakeFile MakeWaitFile %v: %v", fpath, err)
	}
	return nil
}

func (ld *LocalD) removeWaitFile(pid string) error {
	fpath := waitFilePath(pid)
	err := ld.Remove(fpath)
	if err != nil {
		log.Printf("Error on RemoveWaitFile  %v: %v", fpath, err)
		return err
	}
	return nil
}

func (ld *LocalD) setWaitFileStarted(pid string, started bool) {
	ld.LockFile(fslib.LOCKS, waitFilePath(pid))
	defer ld.UnlockFile(fslib.LOCKS, waitFilePath(pid))

	// Get the current contents of the file & its version
	b1, _, err := ld.GetFile(waitFilePath(pid))
	if err != nil {
		log.Printf("Error reading when registerring retstat: %v, %v", waitFilePath(pid), err)
		return
	}
	var wf fslib.WaitFile
	err = json.Unmarshal(b1, &wf)
	if err != nil {
		log.Fatalf("Error unmarshalling waitfile: %v, %v", string(b1), err)
		return
	}
	wf.Started = started
	b2, err := json.Marshal(wf)
	if err != nil {
		log.Printf("Error marshalling waitfile: %v", err)
		return
	}
	// XXX hack around lack of OTRUNC
	for i := 0; i < fslib.WAITFILE_PADDING; i++ {
		b2 = append(b2, ' ')
	}
	_, err = ld.SetFile(waitFilePath(pid), b2, np.NoV)
	if err != nil {
		log.Printf("Error writing when registerring retstat: %v, %v", waitFilePath(pid), err)
	}
}

// Create a randomly-named ephemeral file to mark into which the return status
// will be written.
func (ld *LocalD) makeRetStatFile() string {
	fname := randstr.Hex(16)
	fpath := path.Join(fslib.RET_STAT, fname)
	err := ld.MakeFile(fpath, 0777|np.DMTMP, np.OWRITE, []byte{})
	if err != nil {
		log.Printf("Error creating return status file: %v, %v", fpath, err)
	}
	return fpath
}

// Write back return statuses
func (ld *LocalD) writeBackRetStats(pid string, status string) {
	ld.LockFile(fslib.LOCKS, waitFilePath(pid))
	defer ld.UnlockFile(fslib.LOCKS, waitFilePath(pid))

	b, _, err := ld.GetFile(waitFilePath(pid))
	if err != nil {
		log.Printf("Error reading waitfile in WriteBackRetStats: %v, %v", waitFilePath(pid), err)
		return
	}
	var wf fslib.WaitFile
	err = json.Unmarshal(b, &wf)
	if err != nil {
		log.Printf("Error unmarshalling waitfile: %v, %v, %v", string(b), wf, err)
	}
	for _, p := range wf.RetStatFiles {
		if len(p) > 0 {
			ld.WriteFile(p, []byte(status))
		}
	}
}

// Register that we want a return status written back
func (ld *LocalD) registerRetStatFile(pid string, fpath string) {
	ld.LockFile(fslib.LOCKS, waitFilePath(pid))
	defer ld.UnlockFile(fslib.LOCKS, waitFilePath(pid))

	// Get the current contents of the file & its version
	b1, _, err := ld.GetFile(waitFilePath(pid))
	if err != nil {
		db.DLPrintf("LOCALD", "Error reading when registerring retstat: %v, %v", waitFilePath(pid), err)
		return
	}
	var wf fslib.WaitFile
	err = json.Unmarshal(b1, &wf)
	if err != nil {
		log.Fatalf("Error unmarshalling waitfile: %v, %v", string(b1), err)
		return
	}
	wf.RetStatFiles = append(wf.RetStatFiles, fpath)
	b2, err := json.Marshal(wf)
	if err != nil {
		log.Printf("Error marshalling waitfile: %v", err)
		return
	}
	// XXX hack around lack of OTRUNC
	for i := 0; i < fslib.WAITFILE_PADDING; i++ {
		b2 = append(b2, ' ')
	}
	_, err = ld.SetFile(waitFilePath(pid), b2, np.NoV)
	if err != nil {
		log.Printf("Error writing when registerring retstat: %v, %v", waitFilePath(pid), err)
	}
}

// XXX When we start handling large numbers of lambdas, may be better to stat
// each exit dep individually. For now, this is more efficient (# of RPCs).
// If we know nothing about an exit dep, ignore it by marking it as exited
func (ld *LocalD) pruneExitDeps(a *fslib.Attr) {
	spawned := ld.getSpawnedLambdas()
	for pid, _ := range a.ExitDep {
		if _, ok := spawned[waitFileName(pid)]; !ok {
			a.ExitDep[pid] = true
		}
	}
}

func (ld *LocalD) getSpawnedLambdas() map[string]bool {
	d, err := ld.ReadDir(fslib.SPAWNED)
	if err != nil {
		log.Printf("Error reading spawned dir in pruneExitDeps: %v", err)
	}
	spawned := map[string]bool{}
	for _, l := range d {
		spawned[l.Name] = true
	}
	return spawned
}

func waitFilePath(pid string) string {
	return path.Join(fslib.SPAWNED, waitFileName(pid))
}

func waitFileName(pid string) string {
	return fslib.LockName(fslib.WAIT_LOCK + pid)
}
