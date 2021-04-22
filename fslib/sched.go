package fslib

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	np "ulambda/ninep"
)

const (
	SCHEDQ           = "name/schedq"
	RUNQ             = "runq="
	WAITQ            = "waitq="
	CLAIMED          = "claimed="
	CLAIMED_EPH      = "claimed_ephemeral="
	RUNQ_PATH        = SCHEDQ + "/" + RUNQ
	WAITQ_PATH       = SCHEDQ + "/" + WAITQ
	CLAIMED_PATH     = SCHEDQ + "/" + CLAIMED
	CLAIMED_EPH_PATH = SCHEDQ + "/" + CLAIMED_EPH
	WAIT_LOCK        = "wait-lock."
	JOB_SIGNAL       = "job-signal"
	CRASH_TIMEOUT    = 1
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
	d, err := fl.ReadDir(SCHEDQ)
	if err != nil {
		return d, err
	}
	jobs := filterQueue(RUNQ, d)
	return jobs, err
}

func (fl *FsLib) ReadClaimed() ([]*np.Stat, error) {
	d, err := fl.ReadDir(SCHEDQ)
	if err != nil {
		return d, err
	}
	jobs := filterQueue(CLAIMED, d)
	return jobs, err
}

func (fl *FsLib) ReadWaitQ() ([]*np.Stat, error) {
	d, err := fl.ReadDir(SCHEDQ)
	if err != nil {
		return d, err
	}
	jobs := filterQueue(WAITQ, d)
	return jobs, err
}

func (fl *FsLib) ReadWaitQJob(pid string) ([]byte, error) {
	return fl.ReadFile(WAITQ_PATH + pid)
}

func (fl *FsLib) MarkJobRunnable(pid string) error {
	fl.Rename(WAITQ_PATH+pid, RUNQ_PATH+pid)
	// Notify localds that a job has become runnable
	fl.SignalNewJob()
	return nil
}

// Move a job from CLAIMED to RUNQ
func (fl *FsLib) RestartJob(pid string) error {
	fl.Rename(CLAIMED_PATH+pid, RUNQ_PATH+pid)
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
	_, err := fl.Stat(CLAIMED_EPH_PATH + pid)
	if err != nil {
		stat, err := fl.Stat(CLAIMED_PATH + pid)
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
	return fl.claimJob(RUNQ_PATH, pid)
}

// Claim a job by moving it from the runq to the claimed dir
func (fl *FsLib) ClaimWaitQJob(pid string) ([]byte, bool) {
	return fl.claimJob(WAITQ_PATH, pid)
}

func (fl *FsLib) claimJob(queuePath string, pid string) ([]byte, bool) {
	// Write the file to reset its mtime (to avoid racing with Monitor). Ignore
	// errors in the event we lose the race.
	fl.WriteFile(queuePath+pid, []byte{})
	err := fl.Rename(queuePath+pid, CLAIMED_PATH+pid)
	if err != nil {
		return []byte{}, false
	}
	// Create an ephemeral file to mark that locald hasn't crashed
	fd, err := fl.Create(CLAIMED_EPH_PATH+pid, 0777|np.DMTMP, np.OWRITE)
	if err != nil {
		log.Printf("Error creating ephemeral claimed job file: %v", err)
	}
	fl.Close(fd)
	b, err := fl.ReadFile(CLAIMED_PATH + pid)
	if err != nil {
		log.Printf("Error reading claimed job: %v", err)
		return []byte{}, false
	}
	// We shouldn't hold the "new job" lock while running a lambda/doing work
	fl.SignalNewJob()
	return b, true
}

// Filter out jobs from other queues & partially written jobs. Also, rename to
// remove queue prefix
func filterQueue(queue string, jobs []*np.Stat) []*np.Stat {
	filtered := []*np.Stat{}
	for _, s := range jobs {
		// Filter jobs from other queues, or jobs with in-progress writes
		if strings.Index(s.Name, queue) != 0 {
			continue
		}
		s.Name = s.Name[len(queue):]
		filtered = append(filtered, s)
	}
	return filtered
}

func (fl *FsLib) SwapExitDependency(pids []string) error {
	fromPid := pids[0]
	toPid := pids[1]
	ls, _ := fl.ReadDir(SCHEDQ)
	ls = filterQueue(WAITQ, ls)
	for _, l := range ls {
		// Lock the file
		fl.LockFile(LOCKS, WAITQ+l.Name)
		a, err := fl.ReadFile(WAITQ_PATH + l.Name)
		// May get file not found if someone renamed the file
		if err != nil && err.Error() != "file not found" {
			fl.UnlockFile(LOCKS, WAITQ+l.Name)
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
			err = fl.Remove(WAITQ_PATH + l.Name)
			// May get file not found if someone renamed the file
			if err != nil && err.Error() != "file not found" {
				fl.UnlockFile(LOCKS, WAITQ+l.Name)
				continue
			}
			err = fl.MakeFileAtomic(SCHEDQ, WAITQ+l.Name, b)
			if err != nil {
				log.Fatalf("Error in SwapExitDependency MakeFileAtomic %v: %v", l.Name, err)
				return err
			}
		}
		fl.UnlockFile(LOCKS, WAITQ+l.Name)
	}
	return nil
}

func (fl *FsLib) WakeupExit(pid string) error {
	ls, _ := fl.ReadDir(SCHEDQ)
	ls = filterQueue(WAITQ, ls)
	for _, l := range ls {
		// Lock the file
		fl.LockFile(LOCKS, WAITQ+l.Name)
		a, err := fl.ReadFile(WAITQ_PATH + l.Name)
		// May get file not found if someone renamed the file
		if err != nil && err.Error() != "file not found" {
			fl.UnlockFile(LOCKS, WAITQ+l.Name)
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
			err = fl.Remove(WAITQ_PATH + l.Name)
			// May get file not found if someone renamed the file
			if err != nil && err.Error() != "file not found" {
				fl.UnlockFile(LOCKS, WAITQ+l.Name)
				continue
			}
			if err != nil {
				log.Fatalf("Error in WakeupExit Remove %v: %v", l.Name, err)
				return err
			}
			err = fl.MakeFileAtomic(SCHEDQ, WAITQ+l.Name, b)
			if err != nil {
				log.Fatalf("Error in WakeupExit MakeFileAtomic %v: %v", l.Name, err)
				return err
			}
		}
		fl.UnlockFile(LOCKS, WAITQ+l.Name)
	}

	// Notify localds that a job has become runnable
	fl.SignalNewJob()
	return nil
}

// If we know nothing about an exit dep, ignore it by marking it as exited
func (fl *FsLib) pruneExitDeps(a *Attr) {
	for pid, _ := range a.ExitDep {
		if !fl.HasBeenSpawned(pid) {
			a.ExitDep[pid] = true
		}
	}
}
