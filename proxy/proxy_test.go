package proxy_test

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/test"
)

type Tstate struct {
	*test.Tstate
	cmd *exec.Cmd
}

func initTest(t *testing.T) *Tstate {
	ts := &Tstate{}

	// start named
	ts.Tstate = test.MakeTstatePath(t, np.NAMED)
	sts, err := ts.GetDir(np.NAMED)
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, named.InitDir))

	// start proxy
	ts.cmd = exec.Command("../bin/kernel/proxyd")
	ts.cmd.Stdout = os.Stdout
	ts.cmd.Stderr = os.Stderr
	err = ts.cmd.Start()
	assert.Equal(t, nil, err)

	// mount proxy
	_, err = run("sudo mount -t 9p -o trans=tcp,aname=`whoami`,uname=`whoami`,port=1110 127.0.0.1 /mnt/9p")
	assert.Equal(t, nil, err)

	return ts
}

func (ts *Tstate) cleanup() {
	_, err := run("sudo umount /mnt/9p")
	assert.Equal(ts.T, nil, err)

	err = ts.cmd.Process.Kill()
	assert.Equal(ts.T, nil, err)

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
	assert.Equal(t, nil, err)
	assert.Equal(t, ".statsd\n", string(out))

	out, err = run("cat /mnt/9p/.statsd | grep Nwalk")
	assert.Equal(t, nil, err)
	assert.True(t, strings.Contains(string(out), "Nwalk"))

	out, err = run("echo hello > /mnt/9p/xxx")
	assert.Equal(t, nil, err)

	out, err = run("rm /mnt/9p/xxx")
	assert.Equal(t, nil, err)

	out, err = run("mkdir /mnt/9p/d")
	assert.Equal(t, nil, err)

	out, err = run("echo hello > /mnt/9p/d/xxx")
	assert.Equal(t, nil, err)

	out, err = run("ls /mnt/9p/d | grep 'xxx'")
	assert.Equal(t, nil, err)
	assert.Equal(t, "xxx\n", string(out))

	out, err = run("rm /mnt/9p/d/xxx")
	assert.Equal(t, nil, err)

	out, err = run("rmdir /mnt/9p/d")
	assert.Equal(t, nil, err)

	out, err = run("ls /mnt/9p/xxx")
	assert.NotNil(t, err)

	ts.cleanup()
}

func TestSymlinkPath(t *testing.T) {
	ts := initTest(t)

	dn := "name/d"
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	err = ts.Symlink(fslib.MakeTarget(fslib.Named()), "name/namedself", 0777|np.DMTMP)
	assert.Nil(ts.T, err, "Symlink")

	out, err := run("ls /mnt/9p/namedself")
	assert.Equal(t, nil, err)

	log.Printf("Out: %v\n", out)

	ts.cleanup()
}
