package sigmaclntclnt_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/netsigma"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclntclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

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
	assert.Nil(t, err)
	localIP, err := netsigma.LocalIP()
	assert.Nil(t, err)
	pcfg := proc.NewTestProcEnv(sp.ROOTREALM, "127.0.0.1", localIP, "local-build", false)
	sc, err := sigmaclnt.NewSigmaClntFsLibAPI(pcfg, scc)
	assert.Nil(t, err)
	st, err := sc.Stat("name/")
	assert.Nil(t, err)

	db.DPrintf(db.TEST, "Stat %v err %v\n", st, err)

	cmd.Wait()

	ts.Shutdown()
}
