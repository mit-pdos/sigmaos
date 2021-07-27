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

type Ttype uint32
type Tcore uint32

const (
	T_DEF Ttype = 0
	T_LC  Ttype = 1
	T_BE  Ttype = 2
)

const (
	C_DEF Tcore = 0
)

type Attr struct {
	Pid     string
	Program string
	Dir     string
	Args    []string
	Env     []string
	PairDep []PDep
	ExitDep map[string]bool
	Timer   uint32
	Type    Ttype
	Ncore   Tcore
}

type WaitFile struct {
	Started      bool
	RetStatFiles []string
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
	err := fl.makeWaitFile(a.Pid)
	if err != nil {
		return err
	}
	fl.pruneExitDeps(a)
	b, err := json.Marshal(a)
	if err != nil {
		// Unlock the waiter file if unmarshal failed
		fl.removeWaitFile(a.Pid)
		return err
	}
	err = fl.MakeDirFileAtomic(WAITQ, a.Pid, b)
	if err != nil {
		return err
	}
	// Notify localds that a job has become runnable
	fl.SignalNewJob()
	return nil
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
	_, err := fl.Stat(waitFilePath(pid))
	if err == nil {
		return true
	}
	return false
}

/*
 * PairDep-based lambdas are runnable only if they are the producer (whoever
 * claims and runs the producer will also start the consumer, so we disallow
 * unilaterally claiming the consumer for now), and only once all of their
 * consumers have been started. For now we assume that
 * consumers only have one producer, and the roles of producer and consumer
 * are mutually exclusive. We also expect (though not strictly necessary)
 * that producers only have one consumer each. If this is no longer the case,
 * we should handle oversubscription more carefully.
 */
func (fl *FsLib) Started(pid string) error {
	fl.setWaitFileStarted(pid, true)
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
	fl.writeBackRetStats(pid, status)

	// Release people waiting on this lambda
	return fl.removeWaitFile(pid)
}

// Create a file to read return status from, watch wait file, and return
// contents of retstat file.
func (fl *FsLib) Wait(pid string) ([]byte, error) {
	// XXX We can make return statuses optional to save on RPCs if we don't care
	// about them... right now they require a LOT of RPCs.

	// Make a file in which to receive the return status
	fpath := fl.makeRetStatFile()

	// Communicate the file name to the lambda we're waiting on
	fl.registerRetStatFile(pid, fpath)

	// Wait on the lambda with a watch
	done := make(chan bool)
	err := fl.SetRemoveWatch(waitFilePath(pid), func(p string, err error) {
		if err != nil && err.Error() == "EOF" {
			return
		} else if err != nil {
			log.Printf("Error in wait watch: %v", err)
		}
		done <- true
	})
	// if error, don't wait; the lambda may already have exited.
	if err == nil {
		<-done
	}

	// Read the exit status
	b, _, err := fl.GetFile(fpath)
	if err != nil {
		log.Printf("Error reading retstat file in wait: %v, %v", fpath, err)
		return b, err
	}

	// Clean up our temp file
	err = fl.Remove(fpath)
	if err != nil {
		log.Printf("Error removing retstat file in wait: %v, %v", fpath, err)
		return b, err
	}
	return b, err
}
