package proxy_test

import (
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
	ts.Tstate = test.MakeTstatePath(t, "", np.NAMED)
	sts, err := ts.GetDir(np.NAMED)
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, named.InitDir))

	// start proxy
	ts.cmd = exec.Command("../bin/kernel/proxyd")
	ts.cmd.Stdout = os.Stdout
	err = ts.cmd.Start()
	assert.Equal(t, nil, err)
	return ts
}

func (ts *Tstate) cleanup() {
	shcmd := "sudo umount /mnt/9p"
	err := exec.Command("bash", "-c", shcmd).Run()
	assert.Equal(ts.T, nil, err)

	err = ts.cmd.Process.Kill()
	assert.Equal(ts.T, nil, err)

	ts.Shutdown()
}

func run(cmd string) ([]byte, error) {
	return exec.Command("bash", "-c", cmd).Output()
}

func TestBasic(t *testing.T) {
	ts := initTest(t)

	out, err := run("sudo mount -t 9p -o trans=tcp,aname=`whoami`,uname=`whoami`,port=1110 127.0.0.1 /mnt/9p")
	assert.Equal(t, nil, err)

	out, err = run("ls -a /mnt/9p/ | grep '.statsd'")
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
