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

// XXX TODO: handle network partition; maybe notify client
// that client lost lock because connection failed.

type DLock struct {
	lockDir  string // Path to the lock's parent dir
	lockName string // filename w. "/" characters replaced with "-" characters
	strict   bool   // When true, throws error if lock/unlock fails
	*fslib.FsLib
}

func MakeDLock(fsl *fslib.FsLib, lDir, lName string, strict bool) *DLock {
	l := &DLock{}
	l.lockDir = lDir
	l.lockName = lockName(lName)
	l.FsLib = fsl
	l.strict = strict
	return l
}

// XXX cleanup on failure
func (l *DLock) WeakLock() error {
	fn := path.Join(l.lockDir, l.lockName)
	err := l.MakeFile(fn, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() == "EOF" {
		db.DLPrintf("DLOCK", "Makefile %v err %v", fn, err)
		return err
	}
	if err != nil {
		if l.strict {
			// debug.PrintStack()
			log.Fatalf("%v: MakeFile %v err %v", db.GetName(), fn, err)
		} else {
			log.Printf("%v: Makefile %v err %v", db.GetName(), fn, err)
		}
		return err
	}
	st, err := l.Stat(path.Join(l.lockDir, l.lockName))
	if err != nil {
		db.DLPrintf("DLOCK", "%v: Stat %v err %v", db.GetName(), st, err)
		return err
	}
	err = l.RegisterLock(fn, st.Qid.Version)
	if err != nil {
		db.DLPrintf("DLOCK", "%v: RegisterLock %v err %v", db.GetName(), fn, err)
		return err
	}
	return nil
}

func (l *DLock) Unlock() error {
	fn := path.Join(l.lockDir, l.lockName)
	defer func() error {
		err := l.DeregisterLock(fn)
		if err != nil {
			db.DLPrintf("DLOCK", "%v: DeregisterLock %v err %v", db.GetName(), fn, err)
			return err
		}
		return err
	}()
	err := l.Remove(fn)
	if err != nil {
		if err.Error() == "EOF" {
			db.DLPrintf("DLOCK", "%v: Remove %v err %v", db.GetName(), fn, err)
			return err
		}
		if l.strict {
			debug.PrintStack()
			log.Fatalf("%v: Unlock %v, %v", db.GetName(), fn, err)
		} else {
			db.DLPrintf("DLOCK", "Unlock %v, %v", db.GetName(), fn, err)
		}
	}
	return nil
}

func lockName(f string) string {
	return strings.ReplaceAll(f, "/", "-")
}
