package sync

import (
	"fmt"
	"log"
	"path"
	"sort"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/rand"
)

const (
	COND       = ".cond"
	LOCK       = ".lock"
	SUFFIX_LEN = 16
)

type FilePriorityBag struct {
	lock *Lock
	cond *Cond
	path string
	*fslib.FsLib
}

// Strict lock checking is turned on if this is a true condition variable.
func MakeFilePriorityBag(fsl *fslib.FsLib, bagPath string) *FilePriorityBag {
	fb := &FilePriorityBag{}
	fb.path = bagPath
	fb.lock = MakeLock(fsl, bagPath, LOCK, true)
	fb.cond = MakeCond(fsl, path.Join(bagPath, COND), fb.lock, true)
	fb.FsLib = fsl

	fb.init()

	return fb
}

func (fb *FilePriorityBag) init() error {
	err := fb.Mkdir(fb.path, 0777)
	if err != nil {
		db.DLPrintf("FB", "Error FilePriorityBag.Init MkDir: %v", err)
		return err
	}
	fb.cond.Init()
	return nil
}

// Add a file to the file bag. We assume there is always space for a file to be
// added (producers don't block other than in order to wait for the lock).
func (fb *FilePriorityBag) Put(priority string, name string, contents []byte) error {
	fb.lock.Lock()
	defer fb.lock.Unlock()

	// Add a random suffix to the file name in case of duplicates (but divide by
	// two since each byte will have two characters)
	name = name + rand.String(SUFFIX_LEN/2)

	// XXX Maybe we could avoid doing this every time
	fb.Mkdir(path.Join(fb.path, priority), 0777)

	err := fb.MakeFile(path.Join(fb.path, priority, name), 0777, np.OWRITE, contents)
	if err != nil {
		log.Fatalf("Error MakeFile in FilePriorityBag.Put: %v", err)
		return err
	}

	fb.cond.Signal()

	return nil
}

// Remove a file from the bag. Consumers may block if no file is available.
func (fb *FilePriorityBag) Get() (string, string, []byte, error) {
	fb.lock.Lock()
	defer fb.lock.Unlock()

	var nextPriority string
	var nextName string
	var empty bool

	nextPriority, nextName, empty = fb.isEmptyL()

	// Wait until there are entries available
	for ; empty; nextPriority, nextName, empty = fb.isEmptyL() {
		err := fb.cond.Wait()
		if err != nil {
			return "", "", nil, err
		}
	}

	priority := nextPriority
	name := nextName[:len(nextName)-SUFFIX_LEN]
	contents := fb.removeFileL(nextPriority, nextName)

	return priority, name, contents, nil
}

func (fb *FilePriorityBag) IsEmpty() bool {
	fb.lock.Lock()
	defer fb.lock.Unlock()

	_, _, empty := fb.isEmptyL()
	return empty
}

func (fb *FilePriorityBag) Destroy() {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	fb.cond.Destroy()
}

func (fb *FilePriorityBag) isEmptyL() (string, string, bool) {
	nextP, nextF, err := fb.nextFileL()
	if err != nil {
		return "", "", true
	}
	return nextP, nextF, false
}

func (fb *FilePriorityBag) nextFileL() (string, string, error) {
	priorities, err := fb.ReadDir(fb.path)
	if err != nil {
		if err.Error() != "EOF" {
			log.Fatalf("Error ReadDir 1 in FilePriorityBag.nextFile: %v, %v", fb.path, err)
		}
		db.DLPrintf("FILEBAG", "Error ReadDir 1 in FilePriorityBag.nextFile: %v, %v", fb.path, err)
		return "", "", err
	}

	// Sort the priority buckets
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i].Name < priorities[j].Name
	})

	for _, p := range priorities {
		// Skip the lock file & cond dir.
		if p.Name == LOCK || p.Name == COND {
			continue
		}
		// Read the files in this bucket.
		files, err := fb.ReadDir(path.Join(fb.path, p.Name))
		if err != nil {
			log.Fatalf("Error ReadDir 2 in FilePriorityBag.nextFile: %v, %v", path.Join(fb.path, p.Name), err)
		}
		// Select the first file (guaranteeing no particular order)
		return p.Name, files[0].Name, nil
	}
	return "", "", fmt.Errorf("No files left")
}

// Retrieve a file's contents and remove the file.
func (fb *FilePriorityBag) removeFileL(priority string, name string) []byte {
	fpath := path.Join(fb.path, priority, name)

	contents, err := fb.GetFile(fpath)
	if err != nil {
		log.Fatalf("Error GetFile in FilePriorityBag.removeFileL: %v", err)
	}

	err = fb.Remove(fpath)
	if err != nil {
		log.Fatalf("Error Remove 1 in FilePriorityBag.removeFileL: %v", err)
	}

	// Clean up priority dir if it's now empty
	entries, err := fb.ReadDir(path.Join(fb.path, priority))
	if err != nil {
		log.Fatalf("Error ReadDir in FilePriorityBag.removeFileL: %v", err)
	}

	if len(entries) == 0 {
		err = fb.Remove(path.Join(fb.path, priority))
		if err != nil {
			log.Fatalf("Error Remove 2 in FilePriorityBag.removeFileL: %v", err)
		}
	}
	return contents
}
