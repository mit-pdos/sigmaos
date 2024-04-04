package proxy_test

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/fslib"
	"sigmaos/namesrv"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestCompile(t *testing.T) {
}

type Tstate struct {
	*test.Tstate
	cmd *exec.Cmd
}

func initTest(t1 *test.Tstate) (*Tstate, bool) {
	ts := &Tstate{}

	// start named
	ts.Tstate = t1
	sts, err := ts.GetDir(sp.NAMED)
	assert.Equal(t1.T, nil, err)
	assert.True(t1.T, fslib.Present(sts, namesrv.InitRootDir))

	// start proxy
	ts.cmd = exec.Command("../bin/proxy/proxyd", append([]string{ts.ProcEnv().GetInnerContainerIP().String()})...)
	ts.cmd.Stdout = os.Stdout
	ts.cmd.Stderr = os.Stderr
	err = ts.cmd.Start()
	assert.Nil(t1.T, err)

	// mount proxy
	_, err = run("sudo mount -t 9p -o trans=tcp,aname=`whoami`,uname=`whoami`,port=1110 127.0.0.1 /mnt/9p")

	return ts, assert.Nil(t1.T, err, "Error mount proxy: %v", err)
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

func TestProxyBasic(t *testing.T) {
	t1, err1 := test.NewTstatePath(t, sp.NAMED)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts, ok := initTest(t1)
	defer ts.cleanup()
	if !ok {
		return
	}

	out, err := run("ls -a /mnt/9p/ | grep '.statsd'")
	if !assert.Nil(t, err, "Err run ls: %v", err) {
		return
	}
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

	out, err = run("cp ../tutorial/01_local_dev.md /mnt/9p/ddd/yyy")
	assert.Nil(t, err)

	out, err = run("rm /mnt/9p/ddd/yyy")
	assert.Nil(t, err)

	out, err = run("rmdir /mnt/9p/ddd")
	assert.Nil(t, err)

	out, err = run("ls /mnt/9p/xxx")
	assert.NotNil(t, err)
}

func TestProxyMountPath(t *testing.T) {
	t1, err1 := test.NewTstatePath(t, sp.NAMED)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts, ok := initTest(t1)
	defer ts.cleanup()
	if !ok {
		return
	}

	dn := "name/d"
	err := ts.MkDir(dn, 0777)
	assert.Nil(ts.T, err, "dir")

	m1, err := ts.GetNamedMount()
	assert.Nil(ts.T, err, "MountService: %v", err)
	mnt := sp.NewMount(m1.Addrs(), t1.ProcEnv().GetRealm())
	err = ts.NewMount9P("name/namedself", mnt)
	assert.Nil(ts.T, err, "NewMount9P")

	out, err := run("ls /mnt/9p/namedself")
	assert.Nil(t, err)

	log.Printf("Out: %v\n", string(out))

	ts.Remove("name/namedself")
	ts.RmDir(dn)
}
