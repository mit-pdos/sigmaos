package sync

import (
	"log"
	"path"

	"ulambda/fslib"
)

const ()

type Lock struct {
	lockDir  string // Path to the lock's parent dir
	lockName string // Lock's name
	*fslib.FsLib
}

func MakeLock(fsl *fslib.FsLib, lockDir, lockName string) *Lock {
	l := &Lock{}
	l.lockDir = lockDir
	l.lockName = lockName
	l.FsLib = fsl

	return l
}

func (l *Lock) Lock() {
	err := l.LockFile(l.lockDir, l.lockName)
	if err != nil {
		log.Fatalf("Error Lock.Lock: %v, %v", path.Join(l.lockDir, l.lockName), err)
	}
}

func (l *Lock) Unlock() {
	err := l.UnlockFile(l.lockDir, l.lockName)
	if err != nil {
		log.Fatalf("Error Lock.Unlock: %v, %v", path.Join(l.lockDir, l.lockName), err)
	}
}
