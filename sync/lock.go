package sync

import (
	"log"
	"path"
	"runtime/debug"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
)

const ()

type Lock struct {
	lockDir  string // Path to the lock's parent dir
	lockName string // Lock's name
	strict   bool   // When true, throws error if lock/unlock fails
	*fslib.FsLib
}

func MakeLock(fsl *fslib.FsLib, lockDir, lockName string, strict bool) *Lock {
	l := &Lock{}
	l.lockDir = lockDir
	l.lockName = lockName
	l.FsLib = fsl
	l.strict = strict

	if strings.Contains(lockName, "/") {
		log.Fatalf("Invalid lock name: %v", lockName)
	}

	return l
}

func (l *Lock) Lock() {
	err := l.LockFile(l.lockDir, l.lockName)
	if err != nil {
		if l.strict {
			debug.PrintStack()
			log.Fatalf("Error Lock.Lock: %v, %v", path.Join(l.lockDir, l.lockName), err)
		} else {
			//			log.Printf("Error Lock.Lock: %v, %v", path.Join(l.lockDir, l.lockName), err)
			db.DLPrintf("LOCK", "Error Lock.Lock: %v, %v", path.Join(l.lockDir, l.lockName), err)
		}
	}
}

func (l *Lock) TryLock() bool {
	return l.TryLockFile(l.lockDir, l.lockName)
}

func (l *Lock) Unlock() {
	err := l.UnlockFile(l.lockDir, l.lockName)
	if err != nil {
		if l.strict {
			debug.PrintStack()
			log.Fatalf("Error Lock.Unlock: %v, %v", path.Join(l.lockDir, l.lockName), err)
		} else {
			//			log.Printf("Error Lock.Unlock: %v, %v", path.Join(l.lockDir, l.lockName), err)
			db.DLPrintf("LOCK", "Error Lock.Unlock: %v, %v", path.Join(l.lockDir, l.lockName), err)
		}
	}
}
