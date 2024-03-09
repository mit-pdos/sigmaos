package binsrv_test

import (
	"log"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	db "sigmaos/debug"
	// "sigmaos/binsrv"
)

func TestCompile(t *testing.T) {
}

func run(cmd string) ([]byte, error) {
	out, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Printf("stderr: %v", string(out))
	}
	return out, err
}

type Tstate struct {
	T   *testing.T
	cmd *exec.Cmd
}

func newTstate(t *testing.T) *Tstate {
	ts := &Tstate{T: t}
	ts.cmd = exec.Command("bash", "-c", "sudo ../bin/kernel/fsbind /tmp/binfs")
	ts.cmd.Stdout = os.Stdout
	ts.cmd.Stderr = os.Stderr
	err := ts.cmd.Start()
	assert.Nil(t, err)
	return ts
}

func (ts *Tstate) cleanup() {
	_, err := run("sudo umount /mnt/binfs")
	assert.Nil(ts.T, err)
	err = ts.cmd.Process.Kill()
	assert.Nil(ts.T, err)
}

func TestMount(t *testing.T) {
	ts := newTstate(t)

	time.Sleep(2 * time.Second)

	out, err := run("sudo /bin/ls -a /mnt/binfs")
	assert.Nil(t, err, "ls err %v", err)

	db.DPrintf(db.TEST, "out %q", out)

	out, err = run("sudo cat /mnt/binfs/f")
	assert.Nil(t, err, "cat err %v", err)

	db.DPrintf(db.TEST, "out %q", out)

	ts.cleanup()
}

func TestBin(t *testing.T) {
	ts := newTstate(t)

	time.Sleep(2 * time.Second)

	out, err := run("sudo ls -a /mnt/binfs/matmul")
	assert.Nil(t, err, "ls err %v", err)

	db.DPrintf(db.TEST, "out %q", out)

	out, err = run("sudo sh -c /mnt/binfs/matmul")
	assert.Nil(t, err, "run err %v", err)

	db.DPrintf(db.TEST, "out %q", out)

	ts.cleanup()
}
