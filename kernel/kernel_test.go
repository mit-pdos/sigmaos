package kernel_test

import (
	"log"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/sessp"
	"sigmaos/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

//
// Tests automounting and ephemeral files
//

func TestSymlink1(t *testing.T) {
	ts := test.MakeTstateAll(t)

	// Make a target file
	targetPath := sp.UX + "/~ip/symlink-test-file"
	contents := "symlink test!"
	ts.Remove(targetPath)
	_, err := ts.PutFile(targetPath, 0777, sp.OWRITE, []byte(contents))
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
	_, err = ts.SetFile(linkPath+"/", w, sp.OWRITE, 0)
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
	targetDirPath := sp.UX + "/~ip/dir1"
	targetPath := targetDirPath + "/symlink-test-file"
	contents := "symlink test!"
	ts.Remove(targetPath)
	ts.Remove(targetDirPath)
	err := ts.MkDir(targetDirPath, 0777)
	assert.Nil(t, err, "Creating symlink target dir")
	_, err = ts.PutFile(targetPath, 0777, sp.OWRITE, []byte(contents))
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

	uxs, err := ts.GetDir(sp.UX)
	assert.Nil(t, err, "Error reading ux dir")

	uxip := uxs[0].Name

	// Make a target file
	targetDirPath := sp.UX + "/" + uxip + "/tdir"
	targetPath := targetDirPath + "/target"
	contents := "symlink test!"
	ts.Remove(targetPath)
	ts.Remove(targetDirPath)
	err = ts.MkDir(targetDirPath, 0777)
	assert.Nil(t, err, "Creating symlink target dir")
	_, err = ts.PutFile(targetPath, 0777, sp.OWRITE, []byte(contents))
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
	fsl.ProcessDir(linkDir, func(st *sp.Stat) (bool, error) {
		// Read symlink contents
		fd, err := fsl.Open(linkPath+"/", sp.OREAD)
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
	sts, err := ts.GetDir(sp.PROCD)
	stsExcluded := []*sp.Stat{}
	for _, s := range sts {
		if ok := exclude[path.Join(sp.PROCD, s.Name)]; !ok {
			stsExcluded = append(stsExcluded, s)
		}
	}
	assert.Nil(ts.T, err, sp.PROCD)
	assert.Equal(ts.T, 1, len(stsExcluded))
	name := path.Join(sp.PROCD, stsExcluded[0].Name)
	return name
}

func TestEphemeral(t *testing.T) {
	ts := test.MakeTstateAll(t)

	name1 := procdName(ts, map[string]bool{path.Dir(sp.PROCD_WS): true})

	var err error
	err = ts.BootProcd()
	assert.Nil(t, err, "kernel/procd")

	name := procdName(ts, map[string]bool{path.Dir(sp.PROCD_WS): true, name1: true})
	b, err := ts.GetFile(name)
	assert.Nil(t, err, name)
	_, error := sp.MkMount(b)
	assert.Nil(t, error, "MkMount")

	sts, err := ts.GetDir(name + "/")
	assert.Nil(t, err, name+"/")
	assert.Equal(t, 8, len(sts)) // .statsd, .fences and ctl and running and runqs

	ts.KillOne(sp.PROCD)

	start := time.Now()
	for {
		if time.Since(start) > 3*sp.Conf.Session.TIMEOUT {
			break
		}
		time.Sleep(sp.Conf.Session.TIMEOUT / 10)
		_, err = ts.GetFile(name1)
		if err == nil {
			log.Printf("retry\n")
			continue
		}
		assert.True(t, sessp.IsErrNotfound(err) || sessp.IsErrUnreachable(err), "Wrong err %v", err)
		break
	}
	assert.Greater(t, 3*sp.Conf.Session.TIMEOUT, time.Since(start), "Waiting too long")

	ts.Shutdown()
}
