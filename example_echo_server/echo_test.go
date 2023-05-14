package example_echo_server_test

import (
	"testing"
	"sigmaos/test"
	"sigmaos/rand"
	dbg "sigmaos/debug"
	echo "sigmaos/example_echo_server"
	"sigmaos/protdevclnt"
	"sigmaos/fslib"
	"sigmaos/proc"
	"github.com/stretchr/testify/assert"
	"path"
	"strconv"
)

type TstateEcho struct {
	*test.Tstate
	jobname string
	pid     proc.Tpid
}

func makeTstateEcho(t *testing.T) (*TstateEcho, error) {
	jobname := rand.String(8)
	jobdir := path.Join(echo.DIR_ECHO_SERVER, jobname)
	var err error
	tse := &TstateEcho{}
	tse.jobname = jobname
	tse.Tstate = test.MakeTstateAll(t)
	tse.MkDir(echo.DIR_ECHO_SERVER, 0777)
	if err = tse.MkDir(jobdir, 0777); err != nil {
		return nil, err
	}
	// Start proc
	p := proc.MakeProc("example-echo", []string{strconv.FormatBool(test.Overlays)})
	p.SetNcore(proc.Tcore(1))
	if _, errs := tse.SpawnBurst([]*proc.Proc{p}, 2); len(errs) > 0 {
		dbg.DFatalf("Error burst-spawnn proc %v: %v", p, errs)
		return nil, err
	}
	if err = tse.WaitStart(p.GetPid()); err != nil {
		dbg.DFatalf("Error spawn proc %v: %v", p, err)
		return nil, err
	}
	tse.pid = p.GetPid() 
	return tse, nil
}

func (tse *TstateEcho) Stop() error {
	if err := tse.Evict(tse.pid); err != nil {
		return err
	}
	if _, err := tse.WaitExit(tse.pid); err != nil {
		return err
	}
	return tse.Shutdown()
}

func TestEcho(t *testing.T) {
	// start server
	tse, err := makeTstateEcho(t)
	assert.Nil(t, err, "Test server should start properly")

	// create a RPC client and query server
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{tse.FsLib}, echo.NAMED_ECHO_SERVER)
	assert.Nil(t, err, "RPC client should be created properly")
	arg := echo.EchoRequest{Text: "Hello World!"}
	res := echo.EchoResult{}
	err = pdc.RPC("Echo.Echo", &arg, &res)
	assert.Nil(t, err, "RPC call should succeed")
	assert.Equal(t, "Hello World!", res.Text)

	// Stop server
	assert.Nil(t, tse.Stop())
}
