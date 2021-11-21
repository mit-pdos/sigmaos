package fslib_test

import (
	"log"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/realm"
)

type Tstate struct {
	*fslib.FsLib
	t   *testing.T
	e   *realm.TestEnv
	cfg *realm.RealmConfig
	s   *kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	bin := ".."
	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg
	ts.s = kernel.MakeSystemNamedAddr(bin, cfg.NamedAddr)

	db.Name("fslib_test")
	ts.FsLib = fslib.MakeFsLibAddr("fslibtest", cfg.NamedAddr)
	ts.t = t

	return ts
}

func TestRemoveSimple(t *testing.T) {
	ts := makeTstate(t)

	fn := "name/f"
	d := []byte("hello")
	err := ts.MakeFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Remove(fn)
	assert.Equal(t, nil, err)

	_, err = ts.Stat(fn)
	assert.NotEqual(t, nil, err)

	ts.e.Shutdown()
}

func TestRemoveNonexistent(t *testing.T) {
	ts := makeTstate(t)

	fn := "name/f"
	d := []byte("hello")
	err := ts.MakeFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Remove("name/this-file-does-not-exist")
	assert.NotNil(t, err)
	ts.e.Shutdown()
}

func TestRemovePath(t *testing.T) {
	ts := makeTstate(t)

	err := ts.Mkdir("name/d1", 0777)
	assert.Equal(t, nil, err)
	fn := "name/d1/f"
	d := []byte("hello")
	err = ts.MakeFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	d1, err := ts.ReadFile(fn)
	assert.Equal(t, "hello", string(d1))

	err = ts.Remove(fn)
	assert.Equal(t, nil, err)

	ts.e.Shutdown()
}

func TestRename(t *testing.T) {
	ts := makeTstate(t)
	err := ts.Mkdir("name/d1", 0777)
	assert.Equal(t, nil, err)
	err = ts.Mkdir("name/d2", 0777)
	assert.Equal(t, nil, err)

	fn := "name/d1/f"
	fn1 := "name/d2/g"
	d := []byte("hello")
	err = ts.MakeFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Rename(fn, fn1)
	assert.Equal(t, nil, err)

	d1, err := ts.ReadFile(fn1)
	assert.Equal(t, "hello", string(d1))
	ts.e.Shutdown()
}

func TestRenameAndRemove(t *testing.T) {
	ts := makeTstate(t)
	err := ts.Mkdir("name/d1", 0777)
	assert.Equal(t, nil, err)
	err = ts.Mkdir("name/d2", 0777)
	assert.Equal(t, nil, err)

	fn := "name/d1/f"
	fn1 := "name/d2/g"
	d := []byte("hello")
	err = ts.MakeFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Rename(fn, fn1)
	assert.Equal(t, nil, err)

	d1, err := ts.ReadFile(fn1)
	assert.Equal(t, nil, err)
	assert.Equal(t, "hello", string(d1))

	_, err = ts.Stat(fn1)
	assert.Equal(t, nil, err)

	err = ts.Remove(fn1)
	assert.Equal(t, nil, err)
	ts.e.Shutdown()
}

func TestNonEmpty(t *testing.T) {
	ts := makeTstate(t)
	err := ts.Mkdir("name/d1", 0777)
	assert.Equal(t, nil, err)
	err = ts.Mkdir("name/d2", 0777)
	assert.Equal(t, nil, err)

	fn := "name/d1/f"
	d := []byte("hello")
	err = ts.MakeFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.Remove("name/d1")
	assert.NotNil(t, err, "Remove")

	err = ts.Rename("name/d2", "name/d1")
	assert.NotNil(t, err, "Rename")

	ts.e.Shutdown()
}

func TestCopy(t *testing.T) {
	ts := makeTstate(t)
	d := []byte("hello")
	src := "name/f"
	dst := "name/g"
	err := ts.MakeFile(src, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	err = ts.CopyFile(src, dst)
	assert.Equal(t, nil, err)

	d1, err := ts.ReadFile(dst)
	assert.Equal(t, "hello", string(d1))

	ts.e.Shutdown()
}

func TestDirSimple(t *testing.T) {
	ts := makeTstate(t)
	dn := "name/d"
	err := ts.Mkdir(dn, 0777)
	assert.Equal(t, nil, err)
	b, err := ts.IsDir(dn)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, b)

	d := []byte("hello")
	err = ts.MakeFile(dn+"/f", 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	sts, err := ts.ReadDir(dn)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(sts))
	assert.Equal(t, "f", sts[0].Name)

	err = ts.RmDir(dn)
	_, err = ts.Stat(dn)
	assert.NotEqual(t, nil, err)

	ts.e.Shutdown()
}

func TestDirDot(t *testing.T) {
	ts := makeTstate(t)
	dn := "name/dir0"
	err := ts.Mkdir(dn, 0777)
	assert.Equal(t, nil, err)
	b, err := ts.IsDir(dn + "/.")
	assert.Equal(t, nil, err)
	assert.Equal(t, true, b)
	err = ts.RmDir(dn + "/.")
	assert.NotEqual(t, nil, err)
	err = ts.RmDir(dn)
	_, err = ts.Stat(dn + "/.")
	assert.NotEqual(t, nil, err)
	_, err = ts.Stat("name/.")
	assert.Equal(t, nil, err)
	ts.e.Shutdown()
}

func (ts *Tstate) procdName(t *testing.T, exclude map[string]bool) string {
	sts, err := ts.ReadDir(named.PROCD)
	stsExcluded := []*np.Stat{}
	for _, s := range sts {
		if ok := exclude[path.Join(named.PROCD, s.Name)]; !ok {
			stsExcluded = append(stsExcluded, s)
		}
	}
	assert.Nil(t, err, named.PROCD)
	assert.Equal(t, 1, len(stsExcluded))
	name := path.Join(named.PROCD, stsExcluded[0].Name)
	return name
}

func TestSymlink1(t *testing.T) {
	ts := makeTstate(t)

	// Make a target file
	targetPath := "name/ux/~ip/symlink-test-file"
	contents := "symlink test!"
	ts.Remove(targetPath)
	err := ts.MakeFile(targetPath, 0777, np.OWRITE, []byte(contents))
	assert.Nil(t, err, "Creating symlink target")

	// Read target file
	b, err := ts.ReadFile(targetPath)
	assert.Nil(t, err, "Creating symlink target")
	assert.Equal(t, string(b), contents, "File contents don't match after reading target")

	// Create a symlink
	linkPath := "name/symlink-test"
	err = ts.Symlink(targetPath, linkPath, 0777)
	assert.Nil(t, err, "Creating link")

	// Read symlink contents
	b, err = ts.ReadFile(linkPath + "/")
	assert.Nil(t, err, "Reading linked file")
	assert.Equal(t, contents, string(b), "File contents don't match")

	ts.e.Shutdown()
}

func TestSymlink2(t *testing.T) {
	ts := makeTstate(t)

	// Make a target file
	targetDirPath := "name/ux/~ip/dir1"
	targetPath := targetDirPath + "/symlink-test-file"
	contents := "symlink test!"
	ts.Remove(targetPath)
	ts.Remove(targetDirPath)
	err := ts.Mkdir(targetDirPath, 0777)
	assert.Nil(t, err, "Creating symlink target dir")
	err = ts.MakeFile(targetPath, 0777, np.OWRITE, []byte(contents))
	assert.Nil(t, err, "Creating symlink target")

	// Read target file
	b, err := ts.ReadFile(targetPath)
	assert.Nil(t, err, "Creating symlink target")
	assert.Equal(t, string(b), contents, "File contents don't match after reading target")

	// Create a symlink
	linkDir := "name/dir2"
	linkPath := linkDir + "/symlink-test"
	err = ts.Mkdir(linkDir, 0777)
	assert.Nil(t, err, "Creating link dir")
	err = ts.Symlink(targetPath, linkPath, 0777)
	assert.Nil(t, err, "Creating link")

	// Read symlink contents
	b, err = ts.ReadFile(linkPath + "/")
	assert.Nil(t, err, "Reading linked file")
	assert.Equal(t, contents, string(b), "File contents don't match")

	ts.e.Shutdown()
}

func TestSymlink3(t *testing.T) {
	ts := makeTstate(t)

	uxs, err := ts.ReadDir("name/ux")
	assert.Nil(t, err, "Error reading ux dir")

	uxip := uxs[0].Name

	// Make a target file
	targetDirPath := "name/ux/" + uxip + "/tdir"
	targetPath := targetDirPath + "/target"
	contents := "symlink test!"
	ts.Remove(targetPath)
	ts.Remove(targetDirPath)
	err = ts.Mkdir(targetDirPath, 0777)
	assert.Nil(t, err, "Creating symlink target dir")
	err = ts.MakeFile(targetPath, 0777, np.OWRITE, []byte(contents))
	assert.Nil(t, err, "Creating symlink target")

	// Read target file
	b, err := ts.ReadFile(targetPath)
	assert.Nil(t, err, "Creating symlink target")
	assert.Equal(t, string(b), contents, "File contents don't match after reading target")

	// Create a symlink
	linkDir := "name/ldir"
	linkPath := linkDir + "/link"
	err = ts.Mkdir(linkDir, 0777)
	assert.Nil(t, err, "Creating link dir")
	err = ts.Symlink(targetPath, linkPath, 0777)
	assert.Nil(t, err, "Creating link")

	fsl := fslib.MakeFsLibAddr("abcd", ts.cfg.NamedAddr)
	fsl.ProcessDir(linkDir, func(st *np.Stat) (bool, error) {
		// Read symlink contents
		fd, err := fsl.Open(linkPath+"/", np.OREAD)
		assert.Nil(t, err, "Opening")
		// Read symlink contents again
		b, err = fsl.ReadFile(linkPath + "/")
		assert.Nil(t, err, "Reading linked file")
		assert.Equal(t, contents, string(b), "File contents don't match")

		err = fsl.Close(fd)
		assert.Nil(t, err, "closing linked file")

		return false, nil
	})

	ts.e.Shutdown()
}

func TestCounter(t *testing.T) {
	const N = 10

	ts := makeTstate(t)
	fd, err := ts.CreateFile("name/cnt", 0777|np.DMTMP, np.OWRITE)
	assert.Equal(t, nil, err)
	b := []byte(strconv.Itoa(0))
	_, err = ts.Write(fd, b)
	assert.Equal(t, nil, err)
	err = ts.Close(fd)
	assert.Equal(t, nil, err)

	ch := make(chan int)

	for i := 0; i < N; i++ {
		go func(i int) {
			ntrial := 0
			for {
				ntrial += 1
				b, v, err := ts.GetFile("name/cnt")
				assert.Equal(t, nil, err)
				n, err := strconv.Atoi(string(b))
				assert.Equal(t, nil, err)
				n += 1
				b = []byte(strconv.Itoa(n))
				_, err = ts.SetFile("name/cnt", b, v)
				if err != nil && err.Error() == "Version mismatch" {
					continue
				}
				assert.Equal(t, nil, err)
				break
			}
			// log.Printf("%d: tries %v\n", i, ntrial)
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
	}
	fd, err = ts.Open("name/cnt", np.ORDWR)
	assert.Equal(t, nil, err)
	b, err = ts.Read(fd, 100)
	assert.Equal(t, nil, err)
	n, err := strconv.Atoi(string(b))
	assert.Equal(t, nil, err)

	assert.Equal(t, N, n)

	ts.s.Shutdown()
	ts.e.Shutdown()
}

func TestEphemeral(t *testing.T) {
	const N = 20
	ts := makeTstate(t)

	name1 := ts.procdName(t, map[string]bool{})

	var err error
	err = ts.s.BootProcd()
	assert.Nil(t, err, "bin/kernel/procd")

	name := ts.procdName(t, map[string]bool{name1: true})
	b, err := ts.ReadFile(name)
	assert.Nil(t, err, name)
	assert.Equal(t, true, fsclnt.IsRemoteTarget(string(b)))

	sts, err := ts.ReadDir(name + "/")
	assert.Nil(t, err, name+"/")
	assert.Equal(t, 4, len(sts)) // statsd and ctl

	ts.s.KillOne(named.PROCD)

	n := 0
	for n < N {
		time.Sleep(100 * time.Millisecond)
		_, err = ts.ReadFile(name)
		if err == nil {
			n += 1
			log.Printf("retry\n")
			continue
		}
		assert.Equal(t, true, strings.HasPrefix(err.Error(), "file not found"))
		break
	}
	assert.Greater(t, N, n, "Waiting too long")

	ts.s.Shutdown()
	ts.e.Shutdown()
}

func TestLock(t *testing.T) {
	const N = 20

	ts := makeTstate(t)
	ch := make(chan int)
	acquired := false
	for i := 0; i < N; i++ {
		go func(i int) {
			fsl := fslib.MakeFsLibAddr("fslibtest"+strconv.Itoa(i), ts.cfg.NamedAddr)
			err := fsl.MakeFile("name/lock", 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
			assert.Equal(t, nil, err)
			assert.Equal(t, false, acquired)
			acquired = true
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
		// log.Printf("%d acquired lock\n", j)
		acquired = false
		err := ts.Remove("name/lock")
		assert.Equal(t, nil, err)
	}
	ts.e.Shutdown()
}

func TestLock1(t *testing.T) {
	ts := makeTstate(t)
	ch := make(chan int)
	err := ts.Mkdir("name/locks", 0777)
	assert.NotNil(t, err, "MkDir")

	// Lock the file
	err = ts.MakeFile("name/locks/test-lock", 0777|np.DMTMP, np.OWRITE|np.OCEXEC, []byte{})
	assert.Equal(t, nil, err)
	fsl := fslib.MakeFsLibAddr("fslibtest0", ts.cfg.NamedAddr)
	go func() {
		err := fsl.MakeFile("name/locks/test-lock", 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
		assert.Nil(t, err, "MakeFile")
		ch <- 0
	}()
	time.Sleep(time.Second * 2)
	err = ts.Remove("name/locks/test-lock")
	assert.Nil(t, err, "Remove")
	go func() {
		time.Sleep(2 * time.Second)
		ch <- 1
	}()
	i := <-ch
	assert.Equal(t, 0, i)
	ts.e.Shutdown()
}

func TestLockAfterConnClose(t *testing.T) {
	ts := makeTstate(t)

	lPath := "name/lock-conn-close-test"

	fsl1 := fslib.MakeFsLibAddr("fslibtest-1", ts.cfg.NamedAddr)
	fsl2 := fslib.MakeFsLibAddr("fslibtest-2", ts.cfg.NamedAddr)

	err := fsl1.MakeFile(lPath, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
	assert.Nil(t, err, "Make lock 1")

	go func() {
		// Should wait
		err := fsl2.MakeFile(lPath, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
		assert.Equal(t, err.Error(), "EOF", "Make lock 2")
	}()

	time.Sleep(500 * time.Millisecond)

	// Kill fsl2's connection
	fsl2.Exit()

	// Remove the lock file
	fsl1.Remove(lPath)

	// Try to lock again (should succeed)
	err = fsl1.MakeFile(lPath, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
	assert.Nil(t, err, "Make lock 3")

	ts.e.Shutdown()
}

func TestWatchRemove(t *testing.T) {
	ts := makeTstate(t)

	fn := "name/w"
	err := ts.MakeFile(fn, 0777, np.OWRITE, nil)
	assert.Equal(t, nil, err)

	ch := make(chan bool)
	err = ts.SetRemoveWatch(fn, func(string, error) {
		ch <- true
	})

	err = ts.Remove(fn)
	assert.Equal(t, nil, err)

	<-ch

	ts.e.Shutdown()
}

func TestWatchCreate(t *testing.T) {
	ts := makeTstate(t)

	fn := "name/w"
	ch := make(chan bool)
	_, err := ts.ReadFileWatch(fn, func(string, error) {
		ch <- true
	})
	assert.NotEqual(t, nil, err)
	if err != nil {
		assert.Equal(t, true, strings.HasPrefix(err.Error(), "file not found"))
	}

	err = ts.MakeFile(fn, 0777, np.OWRITE, nil)
	assert.Equal(t, nil, err)

	<-ch

	ts.e.Shutdown()
}

func TestWatchDir(t *testing.T) {
	ts := makeTstate(t)

	fn := "name/d1"
	err := ts.Mkdir(fn, 0777)
	assert.Equal(t, nil, err)

	ch := make(chan bool)
	err = ts.SetDirWatch(fn, func(string, error) {
		ch <- true
	})
	assert.Equal(t, nil, err)

	err = ts.MakeFile(fn+"/x", 0777, np.OWRITE, nil)
	assert.Equal(t, nil, err)

	<-ch

	ts.e.Shutdown()
}

func TestConcurFile(t *testing.T) {
	const N = 20
	ts := makeTstate(t)
	ch := make(chan int)
	for i := 0; i < N; i++ {
		go func(i int) {
			for j := 0; j < 1000; j++ {
				fn := "name/f" + strconv.Itoa(i)
				data := []byte(fn)
				err := ts.MakeFile(fn, 0777, np.OWRITE, data)
				assert.Equal(t, nil, err)
				d, err := ts.ReadFile(fn)
				assert.Equal(t, nil, err)
				assert.Equal(t, len(data), len(d))
				err = ts.Remove(fn)
				assert.Equal(t, nil, err)
			}
			ch <- i
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
	}
	ts.e.Shutdown()
}

const (
	TODO  = "name/todo"
	DONE  = "name/done"
	NFILE = 1000
)

func (ts *Tstate) initfs() {
	err := ts.Mkdir(TODO, 07000)
	assert.Nil(ts.t, err, "Create done")
	err = ts.Mkdir(DONE, 07000)
	assert.Nil(ts.t, err, "Create todo")
}

// Keep renaming files in the todo directory until we failed to rename
// any file
func (ts *Tstate) testRename(fsl *fslib.FsLib, t string) int {
	ok := true
	i := 0
	for ok {
		ok = false
		sts, err := fsl.ReadDir(TODO)
		assert.Nil(ts.t, err, "ReadDir")
		for _, st := range sts {
			err = fsl.Rename(TODO+"/"+st.Name, DONE+"/"+st.Name+"."+t)
			if err == nil {
				i = i + 1
				ok = true
			} else {
				assert.Contains(ts.t, err.Error(), "file not found")
			}
		}
	}
	return i
}

func (ts *Tstate) checkFs() {
	sts, err := ts.ReadDir(DONE)
	assert.Nil(ts.t, err, "ReadDir")
	assert.Equal(ts.t, NFILE, len(sts), "checkFs")
	files := make(map[int]bool)
	for _, st := range sts {
		n := strings.TrimSuffix(st.Name, filepath.Ext(st.Name))
		n = strings.TrimPrefix(n, "job")
		i, err := strconv.Atoi(n)
		assert.Nil(ts.t, err, "Atoi")
		_, ok := files[i]
		assert.Equal(ts.t, false, ok, "map")
		files[i] = true
	}
	for i := 0; i < NFILE; i++ {
		assert.Equal(ts.t, true, files[i], "checkFs")
	}
}

func TestConcurRename(t *testing.T) {
	const N = 20
	ts := makeTstate(t)
	cont := make(chan bool)
	done := make(chan int)
	ts.initfs()

	// start N threads trying to rename files in todo dir
	for i := 0; i < N; i++ {
		fsl := fslib.MakeFsLibAddr("thread"+strconv.Itoa(i), ts.cfg.NamedAddr)
		go func(fsl *fslib.FsLib, t string) {
			n := 0
			for c := true; c; {
				select {
				case c = <-cont:
				default:
					n += ts.testRename(fsl, t)
				}
			}
			done <- n
		}(fsl, strconv.Itoa(i))
	}

	// generate files in the todo dir
	for i := 0; i < NFILE; i++ {
		err := ts.MakeFile(TODO+"/job"+strconv.Itoa(i), 07000, np.OWRITE, []byte{})
		assert.Nil(ts.t, err, "Create job")
	}

	// tell threads we are done with generating files
	n := 0
	for i := 0; i < N; i++ {
		cont <- false
		n += <-done
	}
	assert.Equal(ts.t, NFILE, n, "sum")
	ts.checkFs()
	ts.e.Shutdown()
}
