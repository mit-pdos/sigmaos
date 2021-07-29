package sync

import (
	"log"
	"path"
)

const ()

type Lock struct {
	lockDir  string // Path to the lock's parent dir
	lockName string // Lock's name
}

func MakeLock(lockDir, lockName string) *Lock {
	l := &Lock{}
	l.lockDir = lockDir
	l.lockName = lockName

	return l
}

func (l *Lock) Lock() {
	err := fsl.LockFile(l.lockDir, l.lockName)
	if err != nil {
		log.Fatalf("Error Lock.Lock: %v, %v", path.Join(l.lockDir, l.lockName), err)
	}
}

func (l *Lock) Unlock() {
	err := fsl.UnlockFile(l.lockDir, l.lockName)
	if err != nil {
		log.Fatalf("Error Lock.Unlock: %v, %v", path.Join(l.lockDir, l.lockName), err)
	}
}
