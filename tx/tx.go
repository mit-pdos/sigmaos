package tx

import (
	"log"
	"path"
	"strings"

	"ulambda/atomic"
	"ulambda/fslib"
	np "ulambda/ninep"
)

const (
	COMMIT = "COMMIT"
)

type Tx struct {
	id        string
	begun     bool
	committed bool
	aborted   bool
	stateDir  string
	lockDir   string
	files     map[string]string     // Maps orig fpath -> tmp fpath
	locks     map[string]*sync.Lock // Maps fpath -> lock protecting file
	fsl       *fslib.FsLib
}

// TODO: clean up the state dir
func MakeTx(fsl *fslib.FsLib, id string, stateDir string, lockDir string) *Tx {
	tx := &Tx{}
	tx.id = id
	tx.stateDir = stateDir
	tx.lockDir = lockDir
	tx.files = make(map[string]string)
	tx.locks = make(map[string]*sync.Lock)
	tx.fsl = fsl
	return tx
}

func (tx *Tx) Begin() error {
	if tx.begun {
		return ErrAlreadyBegun()
	}
	tx.begun = true
	return nil
}

func (tx *Tx) Commit() error {
	if err := tx.checkActive(); err != nil {
		return err
	}
	// Write a status file which marks this transaction as committed. This is the
	// linearization point.
	atomic.MakeFileAtomic(tx.fsl, path.Join(tx.stateDir, tx.id), 0777, []byte(COMMIT))
	// Commit all writes.
	for origPath, tmpPath := range tx.files {
		err := tx.fsl.Rename(tmpPath, origPath)
		if err != nil {
			db.DFatalf("Error Rename in Tx.Commit: %v", err)
			return err
		}
	}
	tx.committed = true
	tx.cleanup()
	return nil
}

func (tx *Tx) Abort() error {
	if err := tx.checkActive(); err != nil {
		return err
	}
	tx.aborted = true
	tx.cleanup()
	return nil
}

func (tx *Tx) ReadFile(fpath string) ([]byte, error) {
	if err := tx.checkActive(); err != nil {
		return nil, err
	}
	// Try and lock the resource.
	if ok := tx.lock(fpath); !ok {
		// If there is a conflicting transaction running, abort.
		tx.Abort()
		return nil, ErrAborted()
	}
	return tx.fsl.ReadFile(fpath)
}

func (tx *Tx) WriteFile(fpath string, b []byte) error {
	if err := tx.checkActive(); err != nil {
		return err
	}
	// Try and lock the resource.
	if ok := tx.lock(fpath); !ok {
		// If there is a conflicting transaction running, abort.
		tx.Abort()
		return ErrAborted()
	}
	// Atomically write the file
	tmpPath := tx.origToTmpPath(fpath)
	tx.files[fpath] = tmpPath
	return tx.fsl.MakeFile(tmpPath, 0777, np.OWRITE, b)
}

func (tx *Tx) checkActive() error {
	if !tx.begun {
		return ErrNotBegun()
	}
	if tx.committed {
		return ErrAlreadyCommitted()
	}
	if tx.aborted {
		return ErrAborted()
	}
	return nil
}

func (tx *Tx) lock(fpath string) bool {
	// Skip if we already hold the lock
	if _, ok := tx.locks[fpath]; ok {
		return true
	}
	l := sync.MakeLock(tx.fsl, tx.lockDir, pathToLockName(fpath), true)
	ok := l.TryLock()
	if ok {
		// Another transaction may have crashed after committing but before writing
		// back the new version of this file. If so, finish writing the crashed
		// transaction's result back before claiming ownership of the file.
		tx.tryCommitCrashedTx(fpath)
	}
	return ok
}

func (tx *Tx) unlock(fpath string) {
	l, ok := tx.locks[fpath]
	if !ok {
		db.DFatalf("Error in Tx.unlock: tried to unlock nonexistent lock")
	}
	l.Unlock()
	delete(tx.locks, fpath)
}

func (tx *Tx) cleanup() error {
	for origPath, tmpPath := range tx.files {
		// If we owned this resource, remove the temp file.
		if _, ok := tx.locks[origPath]; ok {
			tx.fsl.Remove(tmpPath)
			tx.unlock(origPath)
		}
	}
	return nil
}

func (tx *Tx) tryCommitCrashedTx(fpath string) {
	// XXX There is probably a better/more efficient way of doing this.
	var tmpPath string
	matched, err := tx.fsl.ProcessDir(path.Dir(fpath), func(st *np.Stat) (bool, error) {
		if tmpFileMatch(path.Base(fpath), st.Name) {
			tmpPath = st.Name
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		db.DFatalf("Error ProcessDir in Tx.tryCommitCrashedTx: %v", err)
	}

	// If there was an outstanding temp version of this file, check if the
	// transaction was committed.
	if matched {
		status, err := tx.fsl.ReadFile(path.Join(tx.stateDir, tmpPathToTxId(tmpPath)))
		if err == nil && string(status) == COMMIT {
			// If the transaction had committed, finish its write.
			err = tx.fsl.Rename(tmpPath, fpath)
			if err != nil {
				db.DFatalf("Error Rename in Tx.tryCommitCrashedTx: %v", err)
			}
		} else {
			// Otherwise, remove the temp file.
			tx.fsl.Remove(tmpPath)
		}
	}
}

func tmpPathToTxId(tmpPath string) string {
	return strings.Split(tmpPath, "#")[1]
}

func (tx *Tx) origToTmpPath(fpath string) string {
	return fpath + "#" + tx.id
}

// Return true if the candidate is a temp file for fpath
func tmpFileMatch(fpath, candidate string) bool {
	return strings.Contains(candidate, "#") && fpath == strings.Split(candidate, "#")[0]
}

func pathToLockName(fpath string) string {
	return strings.ReplaceAll(fpath, "/", "=")
}
