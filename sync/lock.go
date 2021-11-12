package sync

import (
	"log"
	"path"
	"runtime/debug"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

// XXX TODO: handle network partition with leases.

type Lock struct {
	lockDir  string // Path to the lock's parent dir
	lockName string // Lock's name. "/" characters are replaced with "-" characters
	strict   bool   // When true, throws error if lock/unlock fails
	*fslib.FsLib
}

func MakeLock(fsl *fslib.FsLib, lDir, lName string, strict bool) *Lock {
	l := &Lock{}
	l.lockDir = lDir
	l.lockName = lockName(lName)
	l.FsLib = fsl
	l.strict = strict
	return l
}

func (l *Lock) Lock() {
	err := l.MakeFile(path.Join(l.lockDir, l.lockName), 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() == "EOF" {
		db.DLPrintf("LOCK", "Error Lock.Lock: %v", err)
		return
	}
	if err != nil {
		if l.strict {
			debug.PrintStack()
			log.Fatalf("%v: Error MakeFile in Lock.Lock: %v, %v", db.GetName(), path.Join(l.lockDir, l.lockName), err)
		} else {
			log.Printf("%v: Error MakeFile in Lock.Lock: %v, %v", db.GetName(), path.Join(l.lockDir, l.lockName), err)
		}
		return
	}
}

func (l *Lock) TryLock() bool {
	err := l.MakeFile(path.Join(l.lockDir, l.lockName), 0777|np.DMTMP, np.OWRITE, []byte{})
	// If name exists, someone already has the lock...
	if err != nil && err.Error() == "Name exists" {
		return false
	}
	if err != nil {
		log.Printf("Error MakeFile in Lock.TryLock: %v", err)
		return false
	}
	return true
}

func (l *Lock) Unlock() {
	err := l.Remove(path.Join(l.lockDir, l.lockName))
	if err != nil {
		if err.Error() == "EOF" {
			db.DLPrintf("LOCK", "Error Lock.Unlock: %v", err)
			return
		}
		if l.strict {
			debug.PrintStack()
			log.Fatalf("Error Lock.Unlock: %v, %v", path.Join(l.lockDir, l.lockName), err)
		} else {
			db.DLPrintf("LOCK", "Error Lock.Unlock: %v, %v", path.Join(l.lockDir, l.lockName), err)
		}
	}
}

func lockName(f string) string {
	return strings.ReplaceAll(f, "/", "-")
}
