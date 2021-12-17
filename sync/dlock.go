package sync

import (
	"log"
	"path"
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

func MakeLease(fsl *fslib.FsLib, lName string) *DLock {
	l := &DLock{}
	l.lockName = lName
	l.FsLib = fsl
	return l
}
func (l *DLock) MakeLease(b []byte) error {
	fn := path.Join(l.lockDir, l.lockName)
	err := l.MakeFile(fn, 0777|np.DMTMP, np.OWRITE, b)
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() == "EOF" {
		db.DLPrintf("DLOCK", "Makefile %v err %v", fn, err)
		return err
	}
	if err != nil {
		log.Printf("%v: RegisterLock %v err %v", db.GetName(), l.lockName, err)
		return err
	}
	return nil
}

// XXX cleanup on failure
func (l *DLock) WaitWLease() error {
	err := l.MakeFile(l.lockName, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() == "EOF" {
		db.DLPrintf("DLOCK", "Makefile %v err %v", l.lockName, err)
		return err
	}
	if err != nil {
		log.Printf("%v: Makefile %v err %v", db.GetName(), l.lockName, err)
		return err
	}
	st, err := l.Stat(l.lockName)
	if err != nil {
		log.Printf("%v: Stat %v err %v", db.GetName(), st, err)
		return err
	}
	err = l.RegisterLock(l.lockName, st.Qid)
	if err != nil {
		log.Printf("%v: RegisterLock %v err %v", db.GetName(), l.lockName, err)
		return err
	}
	return nil
}

// XXX remove stat
func (l *DLock) WeakRLease() error {
	fn := path.Join(l.lockDir, l.lockName)
	st, err := l.Stat(path.Join(l.lockDir, l.lockName))
	if err != nil {
		// log.Printf("%v: Stat %v err %v", db.GetName(), st, err)
		return err
	}
	err = l.RegisterLock(fn, st.Qid)
	if err != nil {
		log.Printf("%v: RegisterLock %v err %v", db.GetName(), fn, err)
		return err
	}
	return nil
}

func (l *DLock) InvalidateLease() error {
	fn := path.Join(l.lockDir, l.lockName)
	err := l.Remove(fn)
	if err != nil {
		log.Printf("%v: Remove %v err %v", db.GetName(), fn, err)
		return err
	}
	return nil
}

func (l *DLock) ReleaseLease() error {
	fn := path.Join(l.lockDir, l.lockName)
	err := l.DeregisterLock(fn)
	if err != nil {
		log.Printf("%v: DeregisterLock %v err %v", db.GetName(), fn, err)
		return err
	}
	return nil
}

func (l *DLock) WaitRLease() ([]byte, error) {
	ch := make(chan bool)
	for {
		b, err := l.ReadFileWatch("name/config", func(string, error) {
			ch <- true
		})
		if err != nil {
			<-ch
		} else {
			return b, l.WeakRLease()
		}
	}
}

func (l *DLock) ReleaseWLease() error {
	defer l.ReleaseLease()
	err := l.Remove(l.lockName)
	if err != nil {
		if err.Error() == "EOF" {
			db.DLPrintf("DLOCK", "%v: Remove %v err %v", db.GetName(), l.lockName, err)
			return err
		}
	}
	return nil
}

func lockName(f string) string {
	return strings.ReplaceAll(f, "/", "-")
}
