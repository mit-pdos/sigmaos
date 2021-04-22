package fslib

import (
	"log"
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
func (fl *FsLib) TryLockFile(f string) bool {
	lockName := LockName(f)
	fd, err := fl.CreateFile(LOCKS+"/"+lockName, 0777, np.OWRITE)
	// If name exists, someone already has the lock...
	if err != nil && err.Error() == "Name exists" {
		return false
	}
	err = fl.Close(fd)
	if err != nil {
		log.Fatalf("Error on Close TryLockFile %v: %v", lockName, err)
	}
	return true
}

// Lock a file
func (fl *FsLib) LockFile(f string) error {
	lockName := LockName(f)
	fd, err := fl.CreateFile(LOCKS+"/"+lockName, 0777, np.OWRITE|np.OCEXEC)
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() != "EOF" {
		log.Fatalf("Error on Create LockFile %v: %v", lockName, err)
		return err
	}
	err = fl.Close(fd)
	if err != nil {
		log.Fatalf("Error on Close LockFile %v: %v", lockName, err)
		return err
	}
	return nil
}

// Unlock a file
func (fl *FsLib) UnlockFile(f string) error {
	lockName := LockName(f)
	err := fl.Remove(LOCKS + "/" + lockName)
	// JOB_SIGNALs are sometimes redundant
	if err != nil && f != JOB_SIGNAL {
		log.Printf("Error on Remove LockFile %v: %v", lockName, err)
		return err
	}
	return nil
}

func (fl *FsLib) MakeDirFileAtomic(dir string, fname string, b []byte) error {
	err := fl.MakeFile(dir+"/"+WRITING+fname, 0777, b)
	if err != nil {
		log.Fatalf("Error in MakeFileAtomic %v/%v: %v", dir, fname, err)
		return err
	}
	err = fl.Rename(dir+"/"+WRITING+fname, dir+"/"+fname)
	if err != nil {
		log.Fatalf("Error in MakeFileAtomic rename %v/%v: %v", dir, fname, err)
		return err
	}
	return nil
}
