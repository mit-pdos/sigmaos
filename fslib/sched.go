package fslib

import (
	"encoding/json"
	"fmt"
	"log"
	"path"
	"time"

	"github.com/thanhpk/randstr"

	db "ulambda/debug"
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
	TMP           = "name/tmp"
	JOB_SIGNAL    = "job-signal"
	WAIT_LOCK     = "wait-lock."
	CRASH_TIMEOUT = 1
	// XXX REMOVE
	WAITFILE_PADDING = 1000
)

func (fl *FsLib) WaitForJob() error {
	// Wait for something runnable
	return fl.LockFile(LOCKS, JOB_SIGNAL)
}

// Notify localds that a job has become runnable
func (fl *FsLib) SignalNewJob() error {
	// Needs to be done twice, since someone waiting on the signal will create a
	// new lock file, even if they've crashed
	return fl.UnlockFile(LOCKS, JOB_SIGNAL)
}

func (fl *FsLib) ReadJob(dir string, job string) ([]byte, error) {
	b, _, err := fl.GetFile(path.Join(dir, job))
	return b, err
}

func (fl *FsLib) ReadRunQ(dir string) ([]*np.Stat, error) {
	d, err := fl.ReadDir(dir)
	if err != nil {
		return d, err
	}
	return d, err
}

func (fl *FsLib) ReadClaimed() ([]*np.Stat, error) {
	d, err := fl.ReadDir(CLAIMED)
	if err != nil {
		return d, err
	}
	return d, err
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

func (fl *FsLib) MarkJobRunnable(pid string, t Ttype) error {
	if t == T_LC {
		fl.Rename(path.Join(WAITQ, pid), path.Join(RUNQLC, pid))
	} else {
		fl.Rename(path.Join(WAITQ, pid), path.Join(RUNQ, pid))
	}
	// Notify localds that a job has become runnable
	fl.SignalNewJob()
	return nil
}

// Move a job from CLAIMED to RUNQ
func (fl *FsLib) RestartJob(pid string) error {
	// XXX read CLAIMED to find out if it is LC?
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

// Check if a job crashed. We know this is the case if it is CLAIMED, but
// the corresponding CLAIMED_EPH file is missing (locald crashed). Since, upon
// successful ClaimJob, there is a very short window during with the CLAIMED
// file exists but the CLAIMED_EPH file does not exist, wait a short amount of
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

// Return true if the job has started. This function assumes the job has at
// least been spawned, and hasn't exited yet. If the job file is not present,
// we assume that it has already started (and probably exited).
func (fl *FsLib) JobStarted(pid string) bool {
	fl.LockFile(LOCKS, waitFilePath(pid))
	defer fl.UnlockFile(LOCKS, waitFilePath(pid))

	// Get the current contents of the file & its version
	b, _, err := fl.GetFile(waitFilePath(pid))
	if err != nil {
		db.DLPrintf("LOCALD", "Job file not found JobStarted: %v, %v", waitFilePath(pid), err)
		return true
	}
	var wf WaitFile
	err = json.Unmarshal(b, &wf)
	if err != nil {
		log.Fatalf("Error unmarshalling waitfile: %v, %v", string(b), err)
	}
	return wf.Started
}

// Claim a job by moving it from the runq to the claimed dir
func (fl *FsLib) ClaimRunQJob(dir string, pid string) ([]byte, bool) {
	return fl.claimJob(dir, pid)
}

// Claim a job by moving it from the runq to the claimed dir
func (fl *FsLib) ClaimWaitQJob(pid string) ([]byte, bool) {
	return fl.claimJob(WAITQ, pid)
}

func (fl *FsLib) UpdatePDeps(pid string) ([]PDep, error) {
	fl.LockFile(LOCKS, path.Join(WAITQ, pid))
	defer fl.UnlockFile(LOCKS, path.Join(WAITQ, pid))

	newDeps := []PDep{}

	b, _, err := fl.GetFile(path.Join(WAITQ, pid))
	if err != nil {
		return newDeps, err
	}
	var a Attr
	err = json.Unmarshal(b, &a)
	if err != nil {
		log.Printf("Couldn't unmarshal job in updatePDeps %v: %v", string(b), err)
	}

	for _, dep := range a.PairDep {
		if dep.Consumer == pid {
			if started := fl.JobStarted(dep.Producer); !started {
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
		_, err = fl.SetFile(waitFilePath(pid), b2, np.NoV)
		if err != nil {
			log.Printf("Error writing UpdatePDeps: %v, %v", waitFilePath(pid), err)
		}
	}

	return newDeps, nil
}

func (fl *FsLib) claimJob(queuePath string, pid string) ([]byte, bool) {
	// Write the file to reset its mtime (to avoid racing with Monitor). Ignore
	// errors in the event we lose the race.
	fl.WriteFile(path.Join(queuePath, pid), []byte{})
	err := fl.Rename(path.Join(queuePath, pid), path.Join(CLAIMED, pid))
	if err != nil {
		return []byte{}, false
	}
	// Create an ephemeral file to mark that locald hasn't crashed
	err = fl.MakeFile(path.Join(CLAIMED_EPH, pid), 0777|np.DMTMP, np.OWRITE, []byte{})
	if err != nil {
		log.Printf("Error making ephemeral claimed job file: %v", err)
	}
	b, _, err := fl.GetFile(path.Join(CLAIMED, pid))
	if err != nil {
		log.Printf("Error reading claimed job: %v", err)
		return []byte{}, false
	}
	// We shouldn't hold the "new job" lock while running a lambda/doing work
	fl.SignalNewJob()
	return b, true
}

func (fl *FsLib) modifyExitDependencies(f func(map[string]bool) bool) error {
	ls, _ := fl.ReadDir(WAITQ)
	for _, l := range ls {
		// Lock the file
		fl.LockFile(LOCKS, path.Join(WAITQ, l.Name))
		a, _, err := fl.GetFile(path.Join(WAITQ, l.Name))
		// May get file not found if someone renamed the file
		if err != nil && err.Error() != "file not found" {
			fl.UnlockFile(LOCKS, path.Join(WAITQ, l.Name))
			continue
		}
		if err != nil {
			log.Fatalf("Error in SwapExitDependency GetFile %v: %v", l.Name, err)
			return err
		}
		var attr Attr
		err = json.Unmarshal(a, &attr)
		if err != nil {
			log.Fatalf("Error in SwapExitDependency Unmarshal %v: %v", a, err)
			return err
		}
		changed := f(attr.ExitDep)
		// If the ExitDep map changed, write it back.
		if changed {
			b, err := json.Marshal(attr)
			if err != nil {
				log.Fatalf("Error in SwapExitDependency Marshal %v: %v", b, err)
				return err
			}
			// XXX OTRUNC isn't implemented for memfs yet, so remove & rewrite
			err = fl.Remove(path.Join(WAITQ, l.Name))
			// May get file not found if someone renamed the file
			if err != nil && err.Error() != "file not found" {
				fl.UnlockFile(LOCKS, path.Join(WAITQ, l.Name))
				continue
			}
			err = fl.MakeDirFileAtomic(WAITQ, l.Name, b)
			if err != nil {
				log.Fatalf("Error in SwapExitDependency MakeFileAtomic %v: %v", l.Name, err)
				return err
			}
		}
		fl.UnlockFile(LOCKS, path.Join(WAITQ, l.Name))
	}
	return nil
}

func (fl *FsLib) SwapExitDependency(pids []string) error {
	fromPid := pids[0]
	toPid := pids[1]
	return fl.modifyExitDependencies(func(deps map[string]bool) bool {
		if _, ok := deps[fromPid]; ok {
			deps[toPid] = false
			deps[fromPid] = true
			return true
		}
		return false
	})
}

func (fl *FsLib) WakeupExit(pid string) error {
	err := fl.modifyExitDependencies(func(deps map[string]bool) bool {
		if _, ok := deps[pid]; ok {
			deps[pid] = true
			return true
		}
		return false
	})
	if err != nil {
		return err
	}
	// Notify localds that a job has become runnable
	fl.SignalNewJob()
	return nil
}

func (fl *FsLib) makeWaitFile(pid string) error {
	fpath := waitFilePath(pid)
	var wf WaitFile
	wf.Started = false
	b, err := json.Marshal(wf)
	if err != nil {
		log.Printf("Error marshalling waitfile: %v", err)
	}
	// XXX hack around lack of OTRUNC
	for i := 0; i < WAITFILE_PADDING; i++ {
		b = append(b, ' ')
	}
	// Make a writable, versioned file
	err = fl.MakeFile(fpath, 0777, np.OWRITE, b)
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() != "EOF" {
		return fmt.Errorf("Error on MakeFile MakeWaitFile %v: %v", fpath, err)
	}
	return nil
}

func (fl *FsLib) removeWaitFile(pid string) error {
	fpath := waitFilePath(pid)
	err := fl.Remove(fpath)
	if err != nil {
		log.Printf("Error on RemoveWaitFile  %v: %v", fpath, err)
		return err
	}
	return nil
}

func (fl *FsLib) setWaitFileStarted(pid string, started bool) {
	fl.LockFile(LOCKS, waitFilePath(pid))
	defer fl.UnlockFile(LOCKS, waitFilePath(pid))

	// Get the current contents of the file & its version
	b1, _, err := fl.GetFile(waitFilePath(pid))
	if err != nil {
		log.Printf("Error reading when registerring retstat: %v, %v", waitFilePath(pid), err)
		return
	}
	var wf WaitFile
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
	for i := 0; i < WAITFILE_PADDING; i++ {
		b2 = append(b2, ' ')
	}
	_, err = fl.SetFile(waitFilePath(pid), b2, np.NoV)
	if err != nil {
		log.Printf("Error writing when registerring retstat: %v, %v", waitFilePath(pid), err)
	}
}

// Create a randomly-named ephemeral file to mark into which the return status
// will be written.
func (fl *FsLib) makeRetStatFile() string {
	fname := randstr.Hex(16)
	fpath := path.Join(RET_STAT, fname)
	err := fl.MakeFile(fpath, 0777|np.DMTMP, np.OWRITE, []byte{})
	if err != nil {
		log.Printf("Error creating return status file: %v, %v", fpath, err)
	}
	return fpath
}

// Write back return statuses
func (fl *FsLib) writeBackRetStats(pid string, status string) {
	fl.LockFile(LOCKS, waitFilePath(pid))
	defer fl.UnlockFile(LOCKS, waitFilePath(pid))

	b, _, err := fl.GetFile(waitFilePath(pid))
	if err != nil {
		log.Printf("Error reading waitfile in WriteBackRetStats: %v, %v", waitFilePath(pid), err)
		return
	}
	var wf WaitFile
	err = json.Unmarshal(b, &wf)
	if err != nil {
		log.Printf("Error unmarshalling waitfile: %v, %v, %v", string(b), wf, err)
	}
	for _, p := range wf.RetStatFiles {
		if len(p) > 0 {
			fl.WriteFile(p, []byte(status))
		}
	}
}

// Register that we want a return status written back
func (fl *FsLib) registerRetStatFile(pid string, fpath string) {
	fl.LockFile(LOCKS, waitFilePath(pid))
	defer fl.UnlockFile(LOCKS, waitFilePath(pid))

	// Get the current contents of the file & its version
	b1, _, err := fl.GetFile(waitFilePath(pid))
	if err != nil {
		db.DLPrintf("LOCALD", "Error reading when registerring retstat: %v, %v", waitFilePath(pid), err)
		return
	}
	var wf WaitFile
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
	for i := 0; i < WAITFILE_PADDING; i++ {
		b2 = append(b2, ' ')
	}
	_, err = fl.SetFile(waitFilePath(pid), b2, np.NoV)
	if err != nil {
		log.Printf("Error writing when registerring retstat: %v, %v", waitFilePath(pid), err)
	}
}

// XXX When we start handling large numbers of lambdas, may be better to stat
// each exit dep individually. For now, this is more efficient (# of RPCs).
// If we know nothing about an exit dep, ignore it by marking it as exited
func (fl *FsLib) pruneExitDeps(a *Attr) {
	spawned := fl.getSpawnedLambdas()
	for pid, _ := range a.ExitDep {
		if _, ok := spawned[waitFileName(pid)]; !ok {
			a.ExitDep[pid] = true
		}
	}
}

func (fl *FsLib) getSpawnedLambdas() map[string]bool {
	d, err := fl.ReadDir(SPAWNED)
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
	return path.Join(SPAWNED, waitFileName(pid))
}

func waitFileName(pid string) string {
	return LockName(WAIT_LOCK + pid)
}
