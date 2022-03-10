package kernel_test

import (
	"log"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/crash"
	"ulambda/delay"
	"ulambda/fenceclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/pathclnt"
	"ulambda/test"
)

func TestSymlink1(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Make a target file
	targetPath := "name/ux/~ip/symlink-test-file"
	contents := "symlink test!"
	ts.Remove(targetPath)
	_, err := ts.PutFile(targetPath, 0777, np.OWRITE, []byte(contents))
	assert.Nil(t, err, "Creating symlink target")

	// Read target file
	b, err := ts.GetFile(targetPath)
	assert.Nil(t, err, "GetFile symlink target")
	assert.Equal(t, string(b), contents, "File contents don't match after reading target")

	// Create a symlink
	linkPath := "name/symlink-test"
	err = ts.Symlink([]byte(targetPath), linkPath, 0777)
	assert.Nil(t, err, "Creating link")

	// Read symlink contents
	b, err = ts.GetFile(linkPath + "/")
	assert.Nil(t, err, "Reading linked file")
	assert.Equal(t, contents, string(b), "File contents don't match")

	// Write symlink contents
	w := []byte("overwritten!!")
	_, err = ts.SetFile(linkPath+"/", w, 0)
	assert.Nil(t, err, "Writing linked file")
	assert.Equal(t, contents, string(b), "File contents don't match")

	// Read target file
	b, err = ts.GetFile(targetPath)
	assert.Nil(t, err, "GetFile symlink target")
	assert.Equal(t, string(w), string(b), "File contents don't match after reading target")

	// Remove the target of the symlink
	err = ts.Remove(linkPath + "/")
	assert.Nil(t, err, "remove linked file")

	_, err = ts.GetFile(targetPath)
	assert.NotNil(t, err, "symlink target")

	ts.Shutdown()
}

func TestSymlink2(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Make a target file
	targetDirPath := "name/ux/~ip/dir1"
	targetPath := targetDirPath + "/symlink-test-file"
	contents := "symlink test!"
	ts.Remove(targetPath)
	ts.Remove(targetDirPath)
	err := ts.MkDir(targetDirPath, 0777)
	assert.Nil(t, err, "Creating symlink target dir")
	_, err = ts.PutFile(targetPath, 0777, np.OWRITE, []byte(contents))
	assert.Nil(t, err, "Creating symlink target")

	// Read target file
	b, err := ts.GetFile(targetPath)
	assert.Nil(t, err, "Creating symlink target")
	assert.Equal(t, string(b), contents, "File contents don't match after reading target")

	// Create a symlink
	linkDir := "name/dir2"
	linkPath := linkDir + "/symlink-test"
	err = ts.MkDir(linkDir, 0777)
	assert.Nil(t, err, "Creating link dir")
	err = ts.Symlink([]byte(targetPath), linkPath, 0777)
	assert.Nil(t, err, "Creating link")

	// Read symlink contents
	b, err = ts.GetFile(linkPath + "/")
	assert.Nil(t, err, "Reading linked file")
	assert.Equal(t, contents, string(b), "File contents don't match")

	ts.Shutdown()
}

func TestSymlink3(t *testing.T) {
	ts := test.MakeTstateAll(t)

	uxs, err := ts.GetDir("name/ux")
	assert.Nil(t, err, "Error reading ux dir")

	uxip := uxs[0].Name

	// Make a target file
	targetDirPath := "name/ux/" + uxip + "/tdir"
	targetPath := targetDirPath + "/target"
	contents := "symlink test!"
	ts.Remove(targetPath)
	ts.Remove(targetDirPath)
	err = ts.MkDir(targetDirPath, 0777)
	assert.Nil(t, err, "Creating symlink target dir")
	_, err = ts.PutFile(targetPath, 0777, np.OWRITE, []byte(contents))
	assert.Nil(t, err, "Creating symlink target")

	// Read target file
	b, err := ts.GetFile(targetPath)
	assert.Nil(t, err, "Creating symlink target")
	assert.Equal(t, string(b), contents, "File contents don't match after reading target")

	// Create a symlink
	linkDir := "name/ldir"
	linkPath := linkDir + "/link"
	err = ts.MkDir(linkDir, 0777)
	assert.Nil(t, err, "Creating link dir")
	err = ts.Symlink([]byte(targetPath), linkPath, 0777)
	assert.Nil(t, err, "Creating link")

	fsl := fslib.MakeFsLibAddr("abcd", fslib.Named())
	fsl.ProcessDir(linkDir, func(st *np.Stat) (bool, error) {
		// Read symlink contents
		fd, err := fsl.Open(linkPath+"/", np.OREAD)
		assert.Nil(t, err, "Opening")
		// Read symlink contents again
		b, err = fsl.GetFile(linkPath + "/")
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
	ts := test.MakeTstateAll(t)

	name1 := procdName(ts, map[string]bool{})

	var err error
	err = ts.BootProcd()
	assert.Nil(t, err, "bin/kernel/procd")

	name := procdName(ts, map[string]bool{name1: true})
	b, err := ts.GetFile(name)
	assert.Nil(t, err, name)
	assert.Equal(t, true, pathclnt.IsRemoteTarget(string(b)))

	sts, err := ts.GetDir(name + "/")
	assert.Nil(t, err, name+"/")
	assert.Equal(t, 6, len(sts)) // statsd and ctl and running and runqs

	ts.KillOne(np.PROCD)

	n := 0
	for n < N {
		time.Sleep(100 * time.Millisecond)
		_, err = ts.GetFile(name1)
		if err == nil {
			n += 1
			log.Printf("retry\n")
			continue
		}
		assert.True(t, np.IsErrNotfound(err))
		break
	}
	assert.Greater(t, N, n, "Waiting too long")

	ts.Shutdown()
}

func procdName(ts *test.Tstate, exclude map[string]bool) string {
	sts, err := ts.GetDir(np.PROCD)
	stsExcluded := []*np.Stat{}
	for _, s := range sts {
		if ok := exclude[path.Join(np.PROCD, s.Name)]; !ok {
			stsExcluded = append(stsExcluded, s)
		}
	}
	assert.Nil(ts.T, err, np.PROCD)
	assert.Equal(ts.T, 1, len(stsExcluded))
	name := path.Join(np.PROCD, stsExcluded[0].Name)
	return name
}

func TestFenceW(t *testing.T) {
	ts := test.MakeTstateAll(t)
	fence := "name/l"

	dirux := "name/ux/~ip/outdir"
	ts.MkDir(dirux, 0777)
	ts.Remove(dirux + "/f")

	fsldl := fslib.MakeFsLibAddr("wfence", fslib.Named())

	ch := make(chan bool)
	go func() {
		wfence := fenceclnt.MakeFenceClnt(fsldl, fence, 0, []string{dirux})
		err := wfence.AcquireFenceW([]byte{})
		assert.Nil(t, err, "WriteFence")

		fd, err := fsldl.Create(dirux+"/f", 0777, np.OWRITE)
		assert.Nil(t, err, "Create")

		ch <- true

		log.Printf("partition from named..\n")

		crash.Partition(fsldl)
		delay.Delay(10)

		// fsldl lost lock, and ts should have it by now so
		// this write and read to ux server should fail
		_, err = fsldl.Write(fd, []byte(strconv.Itoa(1)))
		assert.NotNil(t, err, "Write")

		// XXX opened before change, so maybe ok
		//_, err = fsldl.Read(fd, 100)
		//assert.NotNil(t, err, "Write")

		fsldl.Close(fd)

		ch <- true
	}()

	<-ch

	wfence := fenceclnt.MakeFenceClnt(ts.FsLib, fence, 0, []string{dirux})
	err := wfence.AcquireFenceW([]byte{})
	assert.Nil(t, err, "WriteFence")

	<-ch

	fd, err := ts.Open(dirux+"/f", np.OREAD)
	assert.Nil(t, err, "Open")
	b, err := ts.Read(fd, 100)
	assert.Equal(ts.T, 0, len(b))

	ts.Shutdown()
}
