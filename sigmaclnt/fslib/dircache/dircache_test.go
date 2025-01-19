package dircache_test

import (
	"flag"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt/fslib/dircache"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var pathname string // e.g., --path "name/ux/sp.LOCAL/" or  "name/msched/sp.LOCAL/"

func init() {
	flag.StringVar(&pathname, "path", sp.NAMED, "path for file system")
}

func TestCompile(t *testing.T) {
}

func newEntry(n string) (struct{}, error) {
	return struct{}{}, nil
}

func TestDirCache(t *testing.T) {
	ts, err := test.NewTstatePath(t, pathname)
	assert.Nil(t, err)

	dn := filepath.Join(pathname, "d")
	fn := "f"
	err = ts.MkDir(dn, 0777)
	assert.Nil(t, err)

	_, err = ts.PutFile(filepath.Join(dn, fn), 0777, sp.OWRITE, nil)
	assert.Nil(t, err)

	dc := dircache.NewDirCache[struct{}](ts.FsLib, dn, newEntry, nil, db.TEST, db.TEST)

	ns, err := dc.GetEntries()
	assert.Equal(t, ns[0], fn)
	assert.Nil(t, err)

	ts.Shutdown()
}

func runTest(t *testing.T, f func(*testing.T, *test.Tstate, *dircache.DirCache[struct{}]), timeoutSec int) {
	ts, err := test.NewTstateAll(t)
	assert.Nil(t, err, "Error New Tstate: %v", err)

	done := make(chan bool)
	timeout := time.After(time.Duration(timeoutSec) * time.Second)
	go func() {
		testdir := filepath.Join(sp.NAMED, "test")
		err = ts.MkDir(testdir, 0777)
		assert.Nil(t, err)

		dc := dircache.NewDirCache[struct{}](ts.FsLib, testdir, nil, nil, db.TEST, db.TEST)

		f(t, ts, dc)

		err = ts.RmDirEntries(testdir)
		assert.Nil(t, err)

		err = ts.Remove(testdir)
		assert.Nil(t, err)

		done <- true
	}()

	select {
	case <-timeout:
		assert.Fail(t, "Timeout")
	case <-done:
	}

	ts.Shutdown()
}

func TestDirCacheWaitEntryCreated(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, dc *dircache.DirCache[struct{}]) {
		go func() {
			_, err := ts.Create(filepath.Join(dc.Path, "file0"), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}()
		err := dc.WaitEntryCreated("file0", true)
		assert.Nil(t, err)
	}, 10)
}

func TestDirCacheWaitAllEntriesCreated(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, dc *dircache.DirCache[struct{}]) {
		files := []string{"file1", "file2", "file3"}

		go func() {
			for _, file := range files {
				_, err := ts.Create(filepath.Join(dc.Path, file), 0777, sp.OWRITE)
				assert.Nil(t, err)
			}
		}()
		err := dc.WaitAllEntriesCreated(files, true)
		assert.Nil(t, err)
	}, 10)
}

func TestDirCacheWaitEntryRemoved(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, dc *dircache.DirCache[struct{}]) {
		_, err := ts.Create(filepath.Join(dc.Path, "file0"), 0777, sp.OWRITE)
		assert.Nil(t, err)

		go func() {
			err := ts.Remove(filepath.Join(dc.Path, "file0"))
			assert.Nil(t, err)
		}()
		err = dc.WaitEntryRemoved("file0", true)
		assert.Nil(t, err)
	}, 10)
}

func TestDirCacheWaitAllEntriesRemoved(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, dc *dircache.DirCache[struct{}]) {
		files := []string{"file1", "file2", "file3"}
		for _, file := range files {
			_, err := ts.Create(filepath.Join(dc.Path, file), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}

		go func() {
			for _, file := range files {
				err := ts.Remove(filepath.Join(dc.Path, file))
				assert.Nil(t, err)
			}
		}()
		err := dc.WaitAllEntriesRemoved(files, true)
		assert.Nil(t, err)
	}, 10)
}

func TestDirCacheWaitGetEntriesN(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, dc *dircache.DirCache[struct{}]) {
		files := []string{"file4", "file5", "file6"}

		go func() {
			for _, file := range files {
				_, err := ts.Create(filepath.Join(dc.Path, file), 0777, sp.OWRITE)
				assert.Nil(t, err)
			}
		}()
		entries, err := dc.WaitGetEntriesN(3, true)
		assert.Nil(t, err)
		assert.ElementsMatch(t, files, entries)
	}, 10)
}

func TestDirCacheWaitEmpty(t *testing.T) {
	runTest(t, func(t *testing.T, ts *test.Tstate, dc *dircache.DirCache[struct{}]) {
		files := []string{"file4", "file5", "file6"}
		for _, file := range files {
			_, err := ts.Create(filepath.Join(dc.Path, file), 0777, sp.OWRITE)
			assert.Nil(t, err)
		}

		go func() {
			for _, file := range files {
				err := ts.Remove(filepath.Join(dc.Path, file))
				assert.Nil(t, err)
			}
		}()
		err := dc.WaitEmpty(true)
		assert.Nil(t, err)
	}, 10)
}
