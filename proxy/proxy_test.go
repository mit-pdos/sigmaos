package proxy_test

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/fslib"
	"sigmaos/namedv1"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

type Tstate struct {
	*test.Tstate
	cmd *exec.Cmd
}

func initTest(t *testing.T) *Tstate {
	ts := &Tstate{}

	// start named
	ts.Tstate = test.MakeTstatePath(t, sp.NAMED)
	sts, err := ts.GetDir(sp.NAMED)
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, namedv1.InitRootDir))

	// start proxy

	ts.cmd = exec.Command("../bin/linux/proxyd", append([]string{ts.GetLocalIP()})...)
	ts.cmd.Stdout = os.Stdout
	ts.cmd.Stderr = os.Stderr
	err = ts.cmd.Start()
	assert.Nil(t, err)

	// mount proxy
	_, err = run("sudo mount -t 9p -o trans=tcp,aname=`whoami`,uname=`whoami`,port=1110 127.0.0.1 /mnt/9p")
	assert.Nil(t, err)

	return ts
}

func (ts *Tstate) cleanup() {
	_, err := run("sudo umount /mnt/9p")
	assert.Nil(ts.T, err)

	err = ts.cmd.Process.Kill()
	assert.Nil(ts.T, err)

	ts.Shutdown()
}

func run(cmd string) ([]byte, error) {
	out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Printf("stderr: %v", string(out))
	}
	return out, err
}

func TestBasic(t *testing.T) {
	ts := initTest(t)

	out, err := run("ls -a /mnt/9p/ | grep '.statsd'")
	assert.Nil(t, err)
	assert.Equal(t, ".statsd\n", string(out))

	out, err = run("cat /mnt/9p/.statsd | grep Nwalk")
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(out), "Nwalk"))

	out, err = run("echo hello > /mnt/9p/xxx")
	assert.Nil(t, err)

	out, err = run("mv /mnt/9p/xxx /mnt/9p/yyy")
	assert.Nil(t, err)

	out, err = run("rm /mnt/9p/yyy")
	assert.Nil(t, err)

	out, err = run("mkdir /mnt/9p/ddd")
	assert.Nil(t, err)

	out, err = run("echo hello > /mnt/9p/ddd/xxx")
	assert.Nil(t, err)

	out, err = run("ls /mnt/9p/ddd | grep 'xxx'")
	assert.Nil(t, err)
	assert.Equal(t, "xxx\n", string(out))

	out, err = run("rm /mnt/9p/ddd/xxx")
	assert.Nil(t, err)

	out, err = run("rmdir /mnt/9p/ddd")
	assert.Nil(t, err)

	out, err = run("ls /mnt/9p/xxx")
	assert.NotNil(t, err)

	ts.cleanup()
}

func TestSymlinkPath(t *testing.T) {
	ts := initTest(t)

	dn := "name/d"
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	mnt := sp.MkMountService(ts.NamedAddr())
	err = ts.MkMountSymlink9P("name/namedself", mnt)
	assert.Nil(ts.T, err, "MkMountSymlink")

	out, err := run("ls /mnt/9p/namedself")
	assert.Nil(t, err)

	log.Printf("Out: %v\n", string(out))

	ts.cleanup()
}
