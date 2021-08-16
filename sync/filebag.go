package sync

import (
	"log"
	"path"
	"sort"

	"github.com/thanhpk/randstr"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

const (
	COND       = ".cond"
	LOCK       = ".lock"
	SUFFIX_LEN = 16
)

type FileBag struct {
	lock *Lock
	cond *Cond
	path string
	*fslib.FsLib
}

// Strict lock checking is turned on if this is a true condition variable.
func MakeFileBag(fsl *fslib.FsLib, bagPath string) *FileBag {
	fb := &FileBag{}
	fb.path = bagPath
	fb.lock = MakeLock(fsl, bagPath, LOCK, true)
	fb.cond = MakeCond(fsl, path.Join(bagPath, COND), fb.lock)
	fb.FsLib = fsl

	fb.init()

	return fb
}

func (fb *FileBag) init() error {
	fb.cond.Init()

	err := fb.Mkdir(fb.path, 0777)
	if err != nil {
		db.DLPrintf("FB", "Error FileBag.Init MkDir: %v", err)
		return err
	}
	return nil
}

// Add a file to the file bag. We assume there is always space for a file to be
// added (producers don't block other than in order to wait for the lock).
func (fb *FileBag) Put(name string, contents []byte) error {
	fb.lock.Lock()
	defer fb.lock.Unlock()

	suffix := randstr.Hex(SUFFIX_LEN / 2)

	err := fb.MakeFile(path.Join(fb.path, name+suffix), 0777, np.OWRITE, contents)
	if err != nil {
		log.Fatalf("Error MakeFile in FileBag.Put: %v", err)
		return err
	}

	fb.cond.Signal()

	return nil
}

// Remove a file from the bag. Consumers may block if no file is available.
func (fb *FileBag) Get() (string, []byte, error) {
	fb.lock.Lock()
	defer fb.lock.Unlock()

	var entries []*np.Stat
	var empty bool

	entries, empty = fb.isEmptyL()

	// Wait until there are entries available.
	for ; empty; entries, empty = fb.isEmptyL() {
		err := fb.cond.Wait()
		if err != nil {
			return "", nil, err
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	var entry *np.Stat
	for _, e := range entries {
		if e.Name != COND && e.Name != LOCK {
			entry = e
		}
	}

	contents, _, err := fb.GetFile(path.Join(fb.path, entry.Name))

	if err != nil {
		log.Fatalf("Error GetFile in FileBag.Get: %v", err)
	}

	err = fb.Remove(path.Join(fb.path, entry.Name))
	if err != nil {
		log.Fatalf("Error removing in FileBag.Get: %v", err)
	}

	return entry.Name[:len(entry.Name)-SUFFIX_LEN], contents, nil
}

func (fb *FileBag) IsEmpty() bool {
	fb.lock.Lock()
	defer fb.lock.Unlock()

	_, empty := fb.isEmptyL()
	return empty
}

func (fb *FileBag) isEmptyL() ([]*np.Stat, bool) {
	entries, err := fb.ReadDir(fb.path)
	if err != nil {
		log.Fatalf("Error reading filebag dir: %v, %v", fb.path, err)
	}

	// We expect LOCK and COND to be present...
	return entries, len(entries) <= 2
}

func (fb *FileBag) Destroy() {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	fb.cond.Destroy()
}
