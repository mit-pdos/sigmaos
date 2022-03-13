package kernel_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
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
	"ulambda/proc"
	"ulambda/test"
)

//
// Tests automounting, ephemeral files, and a fence with two servers.
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

// Test if a primary cannot write to a fenced server after primary
// fails
func TestOldPrimaryOnce(t *testing.T) {
	ts := test.MakeTstateAll(t)
	fence := "name/l"

	dirux := np.UX + "/~ip/outdir"
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

		// fsldl lost primary status, and ts should have it by
		// now so this write to ux server should fail
		_, err = fsldl.Write(fd, []byte(strconv.Itoa(1)))
		assert.NotNil(t, err, "Write")

		fsldl.Close(fd)

		ch <- true
	}()

	// Wait until other thread is primary
	<-ch

	// When other thread partitions, we become primary and install
	// fence.
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

func TestOldPrimaryConcur(t *testing.T) {
	const (
		N = 2
	)

	ts := test.MakeTstateAll(t)
	pids := []string{}

	// XXX use the same dir independent of machine running proc
	dir := np.UX + "/~ip/outdir/"
	fn := dir + "out"
	ts.RmDir(dir)
	err := ts.MkDir(dir, 0777)
	_, err = ts.PutFile(fn, 0777, np.OWRITE, []byte{})
	assert.Nil(t, err, "putfile")

	for i := 0; i < N; i++ {
		last := ""
		if i == N-1 {
			last = "last"
		}
		p := proc.MakeProc("bin/user/primary", []string{"name/fence", dir, last})
		err = ts.Spawn(p)
		assert.Nil(t, err, "Spawn")

		err = ts.WaitStart(p.Pid)
		assert.Nil(t, err, "WaitStarted")

		pids = append(pids, p.Pid)
	}

	time.Sleep(1000 * time.Millisecond)

	// wait for last one; the other procs cannot communicate exit
	// status to test because test's procdir is in name/
	_, err = ts.WaitExit(pids[len(pids)-1])
	assert.Nil(t, err, "WaitExit")

	b, err := ts.GetFile(fn)
	assert.Nil(t, err, "GetFile")
	r := bytes.NewReader(b)
	m := make(map[string]bool)
	for {
		l, err := binary.ReadVarint(r)
		if err != nil && err == io.EOF {
			break
		}
		data := make([]byte, l)
		_, err = r.Read(data)
		assert.Nil(t, err, "read")

		var pid string
		err = json.Unmarshal(data, &pid)
		assert.Nil(t, err, "unmarshal")

		log.Printf("pid: %v\n", pid)
		_, ok := m[pid]
		assert.False(t, ok, "pid")
		m[pid] = true
	}
	for _, pid := range pids {
		assert.True(t, m[pid], "pids")
	}
	log.Printf("exit\n")

	ts.Shutdown()
}
