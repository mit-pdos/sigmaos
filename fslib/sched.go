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
	TMP           = "name/tmp"
	JOB_SIGNAL    = "job-signal"
	WAIT_LOCK     = "wait-lock."
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

func (fl *FsLib) markConsumersRunnable(pid string) {
	ls, err := fl.ReadWaitQ()
	if err != nil {
		log.Printf("Error reading WaitQ in markConsumersRunnable: %v", err)
	}
	for _, l := range ls {
		a, err := fl.ReadWaitQJob(l.Name)
		if err != nil {
			continue
		}
		var attr Attr
		err = json.Unmarshal(a, &attr)
		if err != nil {
			log.Printf("Error unmarshalling in markConsumersRunnable: %v", err)
		}
		for _, pair := range attr.PairDep {
			if attr.Pid == pair.Producer {
				break
			} else if attr.Pid == pair.Consumer {
				if pair.Producer == pid {
					fl.MarkJobRunnable(attr.Pid)
					break
				}
			} else {
				log.Fatalf("Locald got PairDep-based lambda with lambda not in pair: %v, %v", attr.Pid, pair)
			}
		}
	}
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

func (fl *FsLib) modifyExitDependencies(f func(map[string]bool) bool) error {
	ls, _ := fl.ReadDir(WAITQ)
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

func (fl *FsLib) removeWaitFile(pid string) error {
	fpath := waitFilePath(pid)
	err := fl.Remove(fpath)
	if err != nil {
		log.Fatalf("Error on RemoveWaitFile  %v: %v", fpath, err)
		return err
	}
	return nil
}

// Create a randomly-named ephemeral file to mark into which the return status
// will be written.
func (fl *FsLib) makeRetStatFile() string {
	fname := randstr.Hex(16)
	fpath := path.Join(RET_STAT, fname)
	fd, err := fl.CreateFile(fpath, 0777|np.DMTMP, np.OWRITE)
	if err != nil {
		log.Printf("Error creating return status file: %v, %v", fpath, err)
	}
	fl.Close(fd)
	return fpath
}

// Write back return statuses
func (fl *FsLib) writeBackRetStats(pid string, status string) {
	fl.LockFile(LOCKS, waitFilePath(pid))
	defer fl.UnlockFile(LOCKS, waitFilePath(pid))

	b, err := fl.ReadFile(waitFilePath(pid))
	if err != nil {
		log.Printf("Error reading waitfile in WriteBackRetStats: %v, %v", waitFilePath(pid), err)
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
func (fl *FsLib) registerRetStatFile(pid string, fpath string) {
	fl.LockFile(LOCKS, waitFilePath(pid))
	defer fl.UnlockFile(LOCKS, waitFilePath(pid))

	// Check if the wait file still exists
	_, err := fl.Stat(waitFilePath(pid))
	if err != nil {
		return
	}
	// Shouldn't use versioning since we want writes & reads to be fully atomic.
	// Specifically, if we're writing while a locald which is Exiting() is
	// reading, they could get garbage data.
	b, err := fl.ReadFile(waitFilePath(pid))
	if err != nil {
		log.Printf("Error reading when registerring retstat: %v, %v", waitFilePath(pid), err)
		return
	}
	b = append(b, '\n')
	b = append(b, []byte(fpath)...)
	err = fl.WriteFile(waitFilePath(pid), b)
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
