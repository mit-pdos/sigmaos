package sigmaclntsrv_test

import (
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/sigmaclntclnt"
	"sigmaos/test"
)

type RPCCh struct {
	req io.Writer
	rep io.Reader
}

func TestCompile(t *testing.T) {
}

func TestStat(t *testing.T) {
	ts := test.NewTstateAll(t)

	cmd := exec.Command("../bin/linux/sigmaclntd", []string{}...)
	stdin, err := cmd.StdinPipe()
	assert.Nil(t, err)
	stdout, err := cmd.StdoutPipe()
	assert.Nil(t, err)
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	assert.Nil(t, err)

	scc := sigmaclntclnt.NewSigmaClntClnt(stdin, stdout)

	st, err := scc.Stat("name/")
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Stat %v err %v\n", st, err)

	cmd.Wait()

	ts.Shutdown()
}
