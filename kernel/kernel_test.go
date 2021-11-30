package kernel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/kernel"
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
	ts.s = kernel.MakeSystem(bin, cfg.NamedAddr)

	ts.FsLib = fslib.MakeFsLibAddr("procd_test", cfg.NamedAddr)
	ts.t = t

	return ts
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

	ts.e.Shutdown()
}
