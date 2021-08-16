package fslib

import (
	"log"
	"path"
	"strings"

	np "ulambda/ninep"
)

const (
	LOCKS = "name/locks"
)

func LockName(f string) string {
	return strings.ReplaceAll(f, "/", "-")
}

// Try to lock a file. If the lock was acquired, return true. Else, return false
func (fl *FsLib) TryLockFile(lockDir string, f string) bool {
	lockName := LockName(f)
	err := fl.MakeFile(path.Join(lockDir, lockName), 0777|np.DMTMP, np.OWRITE, []byte{})
	// If name exists, someone already has the lock...
	if err != nil && err.Error() == "Name exists" {
		return false
	}
	return true
}

// Lock a file
func (fl *FsLib) LockFile(lockDir string, f string) error {
	lockName := LockName(f)
	err := fl.MakeFile(path.Join(lockDir, lockName), 0777|np.DMTMP, np.OWRITE|np.OCEXEC, []byte{})
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() == "EOF" {
		return nil
	}
	if err != nil {
		log.Fatalf("Error on Create LockFile %v: %v", lockName, err)
		return err
	}
	return nil
}

// Unlock a file
func (fl *FsLib) UnlockFile(lockDir string, f string) error {
	lockName := LockName(f)
	err := fl.Remove(path.Join(lockDir, lockName))
	return err
}
