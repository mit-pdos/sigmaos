package proxy_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/fslib"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/test"
)

func TestBasic(t *testing.T) {
	ts := test.MakeTstatePath(t, "", np.NAMED)
	sts, err := ts.GetDir(np.NAMED)
	assert.Equal(t, nil, err)
	assert.True(t, fslib.Present(sts, named.InitDir))

	cmd := exec.Command("../bin/kernel/proxyd")
	cmd.Stdout = os.Stdout
	err = cmd.Start()
	assert.Equal(t, nil, err)

	shcmd := "sudo mount -t 9p -o trans=tcp,aname=`whoami`,uname=`whoami`,port=1110 127.0.0.1 /mnt/9p"
	out, err := exec.Command("bash", "-c", shcmd).Output()
	assert.Equal(t, nil, err)

	shcmd = "ls -a /mnt/9p/ | grep '.statsd'"
	out, err = exec.Command("bash", "-c", shcmd).Output()
	assert.Equal(t, nil, err)
	assert.Equal(t, ".statsd\n", string(out))

	shcmd = "sudo umount /mnt/9p"
	err = exec.Command("bash", "-c", shcmd).Run()
	assert.Equal(t, nil, err)

	err = cmd.Process.Kill()
	assert.Equal(t, nil, err)

	ts.Shutdown()
}
