package npproxysrv_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
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
	assert.True(t1.T, sp.Present(sts, namesrv.InitRootDir))

	// start proxy
	ts.cmd = exec.Command("../../bin/npproxy/npproxyd", append([]string{ts.ProcEnv().GetInnerContainerIP().String()})...)
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
	db.DPrintf(db.TEST, "cmd %v", cmd)
	out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
	if err != nil {
		db.DPrintf(db.ERROR, "stderr: %v", string(out))
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

	out, err := run("ls -ld /mnt/9p/")
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "out %v", string(out))

	out, err = run("ls -a /mnt/9p/ | grep '.statsd'")
	if !assert.Nil(t, err, "Err run ls: %v", err) {
		return
	}
	assert.Equal(t, ".pstatsd\n.statsd\n", string(out))

	out, err = run("echo hello > /mnt/9p/xxx")
	assert.Nil(t, err)

	out, err = run("cat /mnt/9p/xxx ")
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(out), "hello"))

	out, err = run("ls -l /mnt/9p/xxx")
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "out %v", string(out))

	out, err = run("mv /mnt/9p/xxx /mnt/9p/yyy")
	assert.Nil(t, err)

	out, err = run("rm /mnt/9p/yyy")
	assert.Nil(t, err)

	out, err = run("mkdir /mnt/9p/ddd")
	assert.Nil(t, err)

	out, err = run("ls -ld /mnt/9p/ddd")
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(out), "mnt/9p/ddd"))

	out, err = run("echo hello > /mnt/9p/ddd/xxx")
	assert.Nil(t, err)

	out, err = run("ls /mnt/9p/ddd | grep 'xxx'")
	assert.Nil(t, err)
	assert.Equal(t, "xxx\n", string(out))

	out, err = run("rm /mnt/9p/ddd/xxx")
	assert.Nil(t, err)

	out, err = run("cat /mnt/9p/ddd/xxx ")
	assert.NotNil(t, err)

	out, err = run("rm /mnt/9p/ddd/xxx")
	assert.NotNil(t, err)

	out, err = run("ls /mnt/9p/xxx")
	assert.NotNil(t, err)

	out, err = run("cp ../../tutorial/01_local_dev.md /mnt/9p/ddd/yyy")
	assert.Nil(t, err)

	out, err = run("rm /mnt/9p/ddd/yyy")
	assert.Nil(t, err)

	out, err = run("rmdir /mnt/9p/ddd")
	assert.Nil(t, err)
}

func TestStats(t *testing.T) {
	t1, err1 := test.NewTstatePath(t, sp.NAMED)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts, ok := initTest(t1)
	defer ts.cleanup()
	if !ok {
		return
	}

	for i := 0; i < 10; i++ {
		out, err := run("cat /mnt/9p/.statsd")
		assert.Nil(t, err)
		assert.True(t, strings.Contains(string(out), "Nwalk"))
	}
}

func TestUx(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts, ok := initTest(t1)
	defer ts.cleanup()
	if !ok {
		return
	}
	ux := "/mnt/9p/ux"
	out, err := run("ls " + ux)
	assert.Nil(t, err)
	dn := filepath.Join(ux, string(out))
	out, err = run("ls " + dn)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(out), "bin"))
}

func TestBoot(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts, ok := initTest(t1)
	defer ts.cleanup()
	if !ok {
		return
	}
	boot := "/mnt/9p/boot"
	out, err := run("ls " + boot)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "boot srv: %v\n", string(out))
	e := strings.Fields(string(out))

	dn := filepath.Join(boot, e[1])
	db.DPrintf(db.TEST, "boot srv dn: %v\n", dn)
	out, err = run("ls " + dn)
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "boot: %v\n", string(out))
}

func TestSelf(t *testing.T) {
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
	_, err = ts.PutFile(filepath.Join(dn, "myfile"), 0777, sp.OWRITE, nil)
	assert.Equal(t, nil, err)

	m1, err := ts.GetNamedEndpoint()
	assert.Nil(ts.T, err, "EndpointService: %v", err)
	mnt := sp.NewEndpoint(sp.INTERNAL_EP, m1.Addrs())
	err = ts.NewMount9P("name/namedself", mnt)
	assert.Nil(ts.T, err, "NewMount9P")

	out, err := run("ls -l /mnt/9p/namedself/d")
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Out: %v\n", string(out))
	assert.True(t, strings.Contains(string(out), "myfile"))

	ts.Remove("name/namedself")
	ts.RmDir(dn)
}
