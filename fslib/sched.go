package fslib

import (
	"encoding/json"
	"log"
	"path"
	"strings"
	"time"

	"github.com/thanhpk/randstr"

	np "ulambda/ninep"
)

const (
	RUNQ    = "name/runq"
	WAITQ   = "name/waitq"
	CLAIMED = "name/claimed"
	// XXX TODO: handle claimed_eph in a special way
	CLAIMED_EPH   = "name/claimed_ephemeral"
	SPAWNED       = "name/spawned"
	RET_STAT      = "name/retstat"
	JOB_SIGNAL    = "job-signal"
	CRASH_TIMEOUT = 1
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

func (fl *FsLib) ReadRunQ() ([]*np.Stat, error) {
	d, err := fl.ReadDir(RUNQ)
	if err != nil {
		return d, err
	}
	jobs := filterQueue(d)
	return jobs, err
}

func (fl *FsLib) ReadClaimed() ([]*np.Stat, error) {
	d, err := fl.ReadDir(CLAIMED)
	if err != nil {
		return d, err
	}
	jobs := filterQueue(d)
	return jobs, err
}

func (fl *FsLib) ReadWaitQ() ([]*np.Stat, error) {
	d, err := fl.ReadDir(WAITQ)
	if err != nil {
		return d, err
	}
	jobs := filterQueue(d)
	return jobs, err
}

func (fl *FsLib) ReadWaitQJob(pid string) ([]byte, error) {
	return fl.ReadFile(path.Join(WAITQ, pid))
}

func (fl *FsLib) MarkJobRunnable(pid string) error {
	fl.Rename(path.Join(WAITQ, pid), path.Join(RUNQ, pid))
	// Notify localds that a job has become runnable
	fl.SignalNewJob()
	return nil
}

// Move a job from CLAIMED to RUNQ
func (fl *FsLib) RestartJob(pid string) error {
	fl.Rename(path.Join(CLAIMED, pid), path.Join(RUNQ, pid))
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

// Claim a job by moving it from the runq to the claimed dir
func (fl *FsLib) ClaimRunQJob(pid string) ([]byte, bool) {
	return fl.claimJob(RUNQ, pid)
}

// Claim a job by moving it from the runq to the claimed dir
func (fl *FsLib) ClaimWaitQJob(pid string) ([]byte, bool) {
	return fl.claimJob(WAITQ, pid)
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
	fd, err := fl.CreateFile(path.Join(CLAIMED_EPH, pid), 0777|np.DMTMP, np.OWRITE)
	if err != nil {
		log.Printf("Error creating ephemeral claimed job file: %v", err)
	}
	fl.Close(fd)
	b, err := fl.ReadFile(path.Join(CLAIMED, pid))
	if err != nil {
		log.Printf("Error reading claimed job: %v", err)
		return []byte{}, false
	}
	// We shouldn't hold the "new job" lock while running a lambda/doing work
	fl.SignalNewJob()
	return b, true
}

// Filter out partially written jobs.
func filterQueue(jobs []*np.Stat) []*np.Stat {
	filtered := []*np.Stat{}
	for _, s := range jobs {
		// Filter jobs with in-progress writes
		if strings.Index(s.Name, WRITING) == 0 {
			continue
		}
		filtered = append(filtered, s)
	}
	return filtered
}

func (fl *FsLib) SwapExitDependency(pids []string) error {
	fromPid := pids[0]
	toPid := pids[1]
	ls, _ := fl.ReadDir(WAITQ)
	ls = filterQueue(ls)
	for _, l := range ls {
		// Lock the file
		fl.LockFile(LOCKS, path.Join(WAITQ, l.Name))
		a, err := fl.ReadFile(path.Join(WAITQ, l.Name))
		// May get file not found if someone renamed the file
		if err != nil && err.Error() != "file not found" {
			fl.UnlockFile(LOCKS, path.Join(WAITQ, l.Name))
			continue
		}
		if err != nil {
			log.Fatalf("Error in SwapExitDependency ReadFile %v: %v", l.Name, err)
			return err
		}
		var attr Attr
		err = json.Unmarshal(a, &attr)
		if err != nil {
			log.Fatalf("Error in SwapExitDependency Unmarshal %v: %v", a, err)
			return err
		}
		// If the fromPid is a dependency, swap it & write back
		if _, ok := attr.ExitDep[fromPid]; ok {
			attr.ExitDep[toPid] = false
			attr.ExitDep[fromPid] = true
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

func (fl *FsLib) WakeupExit(pid string) error {
	ls, _ := fl.ReadDir(WAITQ)
	ls = filterQueue(ls)
	for _, l := range ls {
		// Lock the file
		fl.LockFile(LOCKS, path.Join(WAITQ, l.Name))
		a, err := fl.ReadFile(path.Join(WAITQ, l.Name))
		// May get file not found if someone renamed the file
		if err != nil && err.Error() != "file not found" {
			fl.UnlockFile(LOCKS, path.Join(WAITQ, l.Name))
			continue
		}
		if err != nil {
			log.Fatalf("Error in WakeupExit ReadFile %v: %v", l.Name, err)
			return err
		}
		var attr Attr
		err = json.Unmarshal(a, &attr)
		if err != nil {
			log.Fatalf("Error in WakeupExit Unmarshal %v: %v", a, err)
			return err
		}
		// If the fromPid is a dependency, swap it & write back
		if _, ok := attr.ExitDep[pid]; ok {
			attr.ExitDep[pid] = true
			b, err := json.Marshal(attr)
			if err != nil {
				log.Fatalf("Error in WakeupExit Marshal %v: %v", b, err)
				return err
			}
			// XXX OTRUNC isn't implemented for memfs yet, so remove & rewrite
			err = fl.Remove(path.Join(WAITQ, l.Name))
			// May get file not found if someone renamed the file
			if err != nil && err.Error() != "file not found" {
				fl.UnlockFile(LOCKS, path.Join(WAITQ, l.Name))
				continue
			}
			if err != nil {
				log.Fatalf("Error in WakeupExit Remove %v: %v", l.Name, err)
				return err
			}
			err = fl.MakeDirFileAtomic(WAITQ, l.Name, b)
			if err != nil {
				log.Fatalf("Error in WakeupExit MakeFileAtomic %v: %v", l.Name, err)
				return err
			}
		}
		fl.UnlockFile(LOCKS, path.Join(WAITQ, l.Name))
	}

	// Notify localds that a job has become runnable
	fl.SignalNewJob()
	return nil
}

func (fl *FsLib) MakeWaitFile(pid string) error {
	fpath := WaitFilePath(pid)
	// Make a writable, versioned file
	fd, err := fl.CreateFile(fpath, 0777, np.OWRITE|np.OVERSION)
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() != "EOF" {
		log.Fatalf("Error on Create MakeWaitFile %v: %v", fpath, err)
		return err
	}
	err = fl.Close(fd)
	if err != nil {
		log.Fatalf("Error on Close MakeWaitFile %v: %v", fpath, err)
		return err
	}
	return nil
}

// Create a randomly-named ephemeral file to mark into which the return status
// will be written.
func (fl *FsLib) MakeRetStatFile() string {
	fname := randstr.Hex(16)
	fpath := path.Join(RET_STAT, fname)
	fd, err := fl.CreateFile(fpath, 0777|np.DMTMP, np.OWRITE)
	if err != nil {
		log.Printf("Error creating return status file: %v, %v", fpath, err)
	}
	fl.Close(fd)
	return fpath
}

// XXX WriteFile seems to have nasty errors when the file doesn't exist. Need to clean up.
// Write back return statuses
func (fl *FsLib) WriteBackRetStats(pid string, status string) {
	fl.LockFile(LOCKS, WaitFilePath(pid))
	defer fl.UnlockFile(LOCKS, WaitFilePath(pid))

	b, err := fl.ReadFile(WaitFilePath(pid))
	if err != nil {
		log.Printf("Error reading waitfile in WriteBackRetStats: %v, %v", WaitFilePath(pid), err)
		return
	}

	paths := strings.Split(strings.TrimSpace(string(b)), "\n")
	for _, p := range paths {
		if len(p) > 0 {
			fl.WriteFile(p, []byte(status))
		}
	}
}

// Register that we want a return status written back
func (fl *FsLib) RegisterRetStatFile(pid string, fpath string) {
	fl.LockFile(LOCKS, WaitFilePath(pid))
	defer fl.UnlockFile(LOCKS, WaitFilePath(pid))

	// Check if the wait file still exists
	_, err := fl.Stat(WaitFilePath(pid))
	if err != nil {
		return
	}

	// Shouldn't use versioning since we want writes & reads to be fully atomic.
	// Specifically, if we're writing while a locald which is Exiting() is
	// reading, they could get garbage data.
	b, err := fl.ReadFile(WaitFilePath(pid))
	if err != nil {
		log.Printf("Error reading when registerring retstat: %v, %v", WaitFilePath(pid), err)
		return
	}
	b = append(b, '\n')
	b = append(b, []byte(fpath)...)
	err = fl.WriteFile(WaitFilePath(pid), b)
	if err != nil {
		log.Printf("Error writing when registerring retstat: %v, %v", WaitFilePath(pid), err)
	}
}

// If we know nothing about an exit dep, ignore it by marking it as exited
func (fl *FsLib) pruneExitDeps(a *Attr) {
	for pid, _ := range a.ExitDep {
		if !fl.HasBeenSpawned(pid) {
			a.ExitDep[pid] = true
		}
	}
}

func WaitFilePath(pid string) string {
	return path.Join(SPAWNED, WaitFileName(pid))
}

func WaitFileName(pid string) string {
	return LockName(WAIT_LOCK + pid)
}
