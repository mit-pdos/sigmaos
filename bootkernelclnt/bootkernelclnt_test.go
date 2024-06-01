package bootkernelclnt_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

//
// Tests automounting and ephemeral files with a kernel with all services
//

func TestCompile(t *testing.T) {
}

func TestSymlink1(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	// Make a target file
	targetPath := sp.UX + "/~local/symlink-test-file"
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
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	// Make a target file
	targetDirPath := sp.UX + "/~local/dir1"
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
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

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

	pe := proc.NewAddedProcEnv(ts.ProcEnv())
	sc, err := ts.NewClnt(0, pe)
	assert.Nil(t, err)
	sc.ProcessDir(linkDir, func(st *sp.Stat) (bool, error) {
		// Read symlink contents
		fd, err := sc.Open(linkPath+"/", sp.OREAD)
		assert.Nil(t, err, "Opening")
		// Read symlink contents again
		b, err = sc.GetFile(linkPath + "/")
		assert.Nil(t, err, "Reading linked file")
		assert.Equal(t, contents, string(b), "File contents don't match")

		err = sc.CloseFd(fd)
		assert.Nil(t, err, "closing linked file")

		return false, nil
	})

	ts.Shutdown()
}

func TestEphemeral(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	name := filepath.Join(sp.SCHEDD, "~any")

	var err error

	b, err := ts.GetFile(name)
	assert.Nil(t, err, name)

	// check if b is indeed reasonable mounting file
	_, error := sp.NewEndpointFromBytes(b)
	assert.Nil(t, error, "NewEndpoint")

	db.DPrintf(db.TEST, "Try GetDir on %v", name+"/")
	sts, err := ts.GetDir(name + "/")
	assert.Nil(t, err, name+"/")

	// 5: .statsd, pids, rpc, and running
	db.DPrintf(db.TEST, "entries %v\n", sp.Names(sts))
	assert.Equal(t, 4, len(sts), "Unexpected len(sts) %v != %v:", sp.Names(sts), 4)

	ts.KillOne(sp.SCHEDDREL)

	start := time.Now()
	for {
		if time.Since(start) > 2*fsetcd.LeaseTTL {
			break
		}
		time.Sleep(fsetcd.LeaseTTL / 3 * time.Second)
		_, err = ts.GetFile(name)
		if err == nil {
			db.DPrintf(db.TEST, "retry")
			continue
		}
		assert.True(t, serr.IsErrorUnavailable(err), "Wrong err %v", err)
		break
	}
	assert.Greater(t, 3*sp.Conf.Session.TIMEOUT, time.Since(start), "Waiting too long")

	ts.Shutdown()
}

func TestBootMulti1(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	db.DPrintf(db.TEST, "Boot second node")

	err := ts.BootNode(1)
	assert.Nil(t, err, "Err boot node: %v", err)

	ts.Shutdown()
}

func TestBootMulti2(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	db.DPrintf(db.TEST, "Boot second node")

	err := ts.BootNode(1)
	assert.Nil(t, err, "Err boot node 1: %v", err)
	err = ts.BootNode(1)
	assert.Nil(t, err, "Err boot node 2: %v", err)

	ts.Shutdown()
}
