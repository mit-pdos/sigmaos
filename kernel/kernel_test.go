package kernel_test

import (
	"log"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/pathclnt"
	"ulambda/test"
)

//
// Tests automounting and ephemeral files
//

func TestSymlink1(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Make a target file
	targetPath := np.UX + "/~ip/symlink-test-file"
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
	_, err = ts.SetFile(linkPath+"/", w, np.OWRITE, 0)
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
	targetDirPath := np.UX + "/~ip/dir1"
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

	uxs, err := ts.GetDir(np.UX)
	assert.Nil(t, err, "Error reading ux dir")

	uxip := uxs[0].Name

	// Make a target file
	targetDirPath := np.UX + "/" + uxip + "/tdir"
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

func TestEphemeral(t *testing.T) {
	ts := test.MakeTstateAll(t)

	name1 := procdName(ts, map[string]bool{path.Dir(np.PROCD_WS_AAA): true})

	var err error
	err = ts.BootProcd()
	assert.Nil(t, err, "kernel/procd")

	name := procdName(ts, map[string]bool{path.Dir(np.PROCD_WS_AAA): true, name1: true})
	b, err := ts.GetFile(name)
	assert.Nil(t, err, name)
	assert.Equal(t, true, pathclnt.IsRemoteTarget(string(b)))

	sts, err := ts.GetDir(name + "/")
	assert.Nil(t, err, name+"/")
	assert.Equal(t, 8, len(sts)) // .statsd, .fences and ctl and running and runqs

	ts.KillOne(np.PROCD)

	start := time.Now()
	for {
		if time.Since(start) > 3*np.Conf.Session.TIMEOUT {
			break
		}
		time.Sleep(np.Conf.Session.TIMEOUT / 10)
		_, err = ts.GetFile(name1)
		if err == nil {
			log.Printf("retry\n")
			continue
		}
		assert.True(t, np.IsErrNotfound(err))
		break
	}
	assert.Greater(t, 3*np.Conf.Session.TIMEOUT, time.Since(start), "Waiting too long")

	ts.Shutdown()
}
