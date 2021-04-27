package fslib

import (
	"encoding/json"
	"log"
	"math/rand"
	"path"
	"strconv"
)

type PDep struct {
	Producer string
	Consumer string
}

type Attr struct {
	Pid     string
	Program string
	Dir     string
	Args    []string
	Env     []string
	PairDep []PDep
	ExitDep map[string]bool
	Timer   uint32
}

const (
	LOCALD_ROOT  = "name/localds"
	NO_OP_LAMBDA = "no-op-lambda"
)

func GenPid() string {
	return strconv.Itoa(rand.Intn(100000))
}

// Spawn a new lambda
func (fl *FsLib) Spawn(a *Attr) error {
	// Create a file for waiters to watch & wait on
	err := fl.MakeWaitFile(a.Pid)
	if err != nil {
		return err
	}
	fl.pruneExitDeps(a)
	b, err := json.Marshal(a)
	if err != nil {
		// Unlock the waiter file if unmarshal failed
		fl.UnlockFile(LOCKS, WAIT_LOCK+a.Pid)
		return err
	}
	err = fl.MakeDirFileAtomic(WAITQ, a.Pid, b)
	if err != nil {
		return err
	}
	// Notify localds that a job has become runnable
	fl.UnlockFile(LOCKS, JOB_SIGNAL)
	return nil
}

func (fl *FsLib) SpawnProgram(name string, args []string) error {
	a := &Attr{}
	a.Pid = GenPid()
	a.Program = name
	a.Args = args
	return fl.Spawn(a)
}

// Spawn a no-op lambda
func (fl *FsLib) SpawnNoOp(pid string, exitDep []string) error {
	a := &Attr{}
	a.Pid = pid
	a.Program = NO_OP_LAMBDA
	exitDepMap := map[string]bool{}
	for _, dep := range exitDep {
		exitDepMap[dep] = false
	}
	a.ExitDep = exitDepMap
	return fl.Spawn(a)
}

// XXX may need to change this
func (fl *FsLib) HasBeenSpawned(pid string) bool {
	_, err := fl.Stat(WaitFilePath(pid))
	if err == nil {
		return true
	}
	return false
}

func (fl *FsLib) Started(pid string) error {
	// TODO: update Status, start consumers, etc.
	// return fl.WriteFile(SCHED+"/"+pid+"/Status", []byte{})
	return nil
}

func (fl *FsLib) Exiting(pid string, status string) error {
	fl.WakeupExit(pid)
	err := fl.Remove(path.Join(CLAIMED, pid))
	if err != nil {
		log.Printf("Error removing claimed in Exiting %v: %v", pid, err)
	}
	err = fl.Remove(path.Join(CLAIMED_EPH, pid))
	if err != nil {
		log.Printf("Error removing claimed_eph in Exiting %v: %v", pid, err)
	}
	// Write back return statuses
	fl.WriteBackRetStats(pid, status)
	err = fl.Remove(WaitFilePath(pid))
	if err != nil {
		log.Printf("Error removing wait file in Exiting %v: %v", pid, err)
	}
	return nil
}

// Create a file to read return status from, watch wait file, and return
// contents of retstat file.
func (fl *FsLib) Wait(pid string) ([]byte, error) {
	fpath := fl.MakeRetStatFile()
	fl.RegisterRetStatFile(pid, fpath)
	done := make(chan bool)
	fl.SetRemoveWatch(WaitFilePath(pid), func(p string, err error) {
		if err != nil && err.Error() == "EOF" {
			return
		} else if err != nil {
			log.Printf("Error in wait watch: %v", err)
		}
		done <- true
	})
	<-done
	b, err := fl.ReadFile(fpath)
	if err != nil {
		log.Printf("Error reading retstat file in wait: %v, %v", fpath, err)
		return b, err
	}
	err = fl.Remove(fpath)
	if err != nil {
		log.Printf("Error removing retstat file in wait: %v, %v", fpath, err)
		return b, err
	}
	return b, err
}
