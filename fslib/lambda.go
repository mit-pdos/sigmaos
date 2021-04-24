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
	WAIT_LOCK    = "wait-lock."
)

func GenPid() string {
	return strconv.Itoa(rand.Intn(100000))
}

// Spawn a new lambda
func (fl *FsLib) Spawn(a *Attr) error {
	// Create a lock file for waiters to wait on
	fl.LockFile(LOCKS, WAIT_LOCK+a.Pid)
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

func (fl *FsLib) HasBeenSpawned(pid string) bool {
	_, err := fl.Stat(path.Join(LOCKS, LockName(WAIT_LOCK+pid)))
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
	err = fl.Remove(path.Join(CLAIMED, pid))
	if err != nil {
		log.Printf("Error removing claimed in Exiting %v: %v", pid, err)
	}
	err = fl.Remove(path.Join(CLAIMED_EPH, pid))
	if err != nil {
		log.Printf("Error removing claimed_eph in Exiting %v: %v", pid, err)
	}
	err = fl.UnlockFile(LOCKS, WAIT_LOCK+pid)
	if err != nil {
		log.Printf("Error unlocking in Exiting %v: %v", pid, err)
	}
	return nil
}

// First check waitq, then runq, then the claimed dir
func (fl *FsLib) Wait(pid string) ([]byte, error) {
	fl.LockFile(LOCKS, WAIT_LOCK+pid)
	fl.UnlockFile(LOCKS, WAIT_LOCK+pid)
	// XXX Return an actual exit status
	return []byte{'O', 'K'}, nil
}
