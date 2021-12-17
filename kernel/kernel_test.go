package kernel_test

import (
	"log"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/crash"
	"ulambda/delay"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/named"
	np "ulambda/ninep"
	usync "ulambda/sync"
)

type Tstate struct {
	*fslib.FsLib
	t *testing.T
	s *kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.s = kernel.MakeSystemAll("..")
	ts.FsLib = fslib.MakeFsLibAddr("kernel_test", fslib.Named())
	return ts
}

func (ts *Tstate) Shutdown() {
	ts.s.Shutdown()
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

	ts.Shutdown()
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

	ts.Shutdown()
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

	fsl := fslib.MakeFsLibAddr("abcd", fslib.Named())
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

	ts.Shutdown()
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
	assert.Equal(t, 5, len(sts)) // statsd and ctl and running and runqs

	ts.s.KillOne(named.PROCD)

	n := 0
	for n < N {
		time.Sleep(100 * time.Millisecond)
		_, err = ts.ReadFile(name1)
		if err == nil {
			n += 1
			log.Printf("retry\n")
			continue
		}
		assert.Equal(t, true, strings.HasPrefix(err.Error(), "file not found"))
		break
	}
	assert.Greater(t, N, n, "Waiting too long")

	ts.Shutdown()
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

func TestWLease(t *testing.T) {
	ts := makeTstate(t)
	lease := "name/l"

	dirux := "name/ux/~ip/outdir"
	ts.Mkdir(dirux, 0777)
	ts.Remove(dirux + "/f")

	fsldl := fslib.MakeFsLibAddr("dlock", fslib.Named())

	ch := make(chan bool)
	go func() {
		wlease := usync.MakeLease(fsldl, lease)
		err := wlease.WaitWLease()
		assert.Nil(t, err, "WriteLease")

		fd, err := fsldl.Create(dirux+"/f", 0777, np.OWRITE)
		assert.Nil(t, err, "Create")

		ch <- true

		log.Printf("partition..\n")

		crash.Partition(fsldl)
		delay.Delay(10)

		// fsldl lost lock, and ts should have it by now so
		// this write should fail
		_, err = fsldl.Write(fd, []byte(strconv.Itoa(1)))
		assert.NotNil(t, err, "Write")

		fsldl.Close(fd)

		ch <- true
	}()

	<-ch

	wlease := usync.MakeLease(ts.FsLib, lease)
	err := wlease.WaitWLease()
	assert.Nil(t, err, "Weaklock")

	<-ch

	fd, err := ts.Open(dirux+"/f", np.OREAD)
	assert.Nil(t, err, "Open")
	b, err := ts.Read(fd, 100)
	assert.Equal(ts.t, 0, len(b))

	ts.Shutdown()
}
