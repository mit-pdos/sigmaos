package example_echo_server_test

import (
	"github.com/stretchr/testify/assert"
	"path/filepath"
	dbg "sigmaos/debug"
	echo "sigmaos/example_echo_server"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/rpcclnt"
	"sigmaos/test"
	"strconv"
	"testing"
)

type TstateEcho struct {
	*test.Tstate
	jobname string
	pid     sp.Tpid
}

func newTstateEcho(t *testing.T) (*TstateEcho, error) {
	jobname := rand.String(8)
	jobdir := filepath.Join(echo.DIR_ECHO_SERVER, jobname)
	var err error
	tse := &TstateEcho{}
	tse.jobname = jobname
	tse.Tstate = test.NewTstateAll(t)
	tse.MkDir(echo.DIR_ECHO_SERVER, 0777)
	if err = tse.MkDir(jobdir, 0777); err != nil {
		return nil, err
	}
	// Start proc
	p := proc.NewProc("example-echo", []string{strconv.FormatBool(test.Overlays)})
	p.SetMcpu(proc.Tmcpu(1000))
	if _, errs := tse.SpawnBurst([]*proc.Proc{p}, 2); !assert.True(t, len(errs) > 0, "Errors spawnBurst: %v", errs) {
		dbg.DPrintf(dbg.ERROR, "Error burst-spawnn proc %v: %v", p, errs)
		return nil, err
	}
	if err = tse.WaitStart(p.GetPid()); !assert.Nil(t, err, "Error spawn proc: %v", nil) {
		dbg.DPrintf(dbg.ERROR, "Error spawn proc %v: %v", p, err)
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
	tse, err := newTstateEcho(t)
	assert.Nil(t, err, "Test server should start properly %v", err)

	// create a RPC client and query server
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{tse.FsLib}, echo.NAMED_ECHO_SERVER)
	assert.Nil(t, err, "RPC client should be created properly")
	arg := echo.EchoRequest{Text: "Hello World!"}
	res := echo.EchoResult{}
	err = rpcc.RPC("EchoSrv.Echo", &arg, &res)
	assert.Nil(t, err, "RPC call should succeed")
	assert.Equal(t, "Hello World!", res.Text)

	// Stop server
	assert.Nil(t, tse.Stop())
}

func TestEchoTime(t *testing.T) {
	// start server
	tse, err := newTstateEcho(t)
	assert.Nil(t, err, "Test server should start properly")

	// create a RPC client and query server
	pdc, err := protdevclnt.NewProtDevClnt([]*fslib.FsLib{tse.FsLib}, echo.NAMED_ECHO_SERVER)
	assert.Nil(t, err, "RPC client should be created properly")
	arg := echo.EchoRequest{Text: "Hello World!"}
	res := echo.EchoResult{}
	N_REQ := 10000
	t0 := time.Now()
	for i := 0; i < N_REQ; i++ {
		pdc.RPC("Echo.Echo", &arg, &res)
	}
	totalTime := time.Since(t0).Microseconds()
	dbg.DPrintf(dbg.ALWAYS, "Total time: %v ms; avg time %v ms", totalTime, totalTime/int64(N_REQ))

	// Stop server
	assert.Nil(t, tse.Stop())

}

func TestEchoLoad(t *testing.T) {
	// start server
	tse, err := newTstateEcho(t)
	assert.Nil(t, err, "Test server should start properly")

	// create a RPC client and query server
	fsls := make([]*fslib.FsLib, 0, N_RPC_SESSIONS)
	for i := 0; i < N_RPC_SESSIONS; i++ {
		fsl, err := fslib.NewFsLib(tse.jobname + "-" + strconv.Itoa(i))
		if !assert.Nil(t, err, "Error newfsl: %v", nil) {
			dbg.DPrintf(dbg.ERROR, "Error newfsl: %v", err)
		}
		fsls = append(fsls, fsl)
	}
	pdc, err := protdevclnt.NewProtDevClnt(fsls, echo.NAMED_ECHO_SERVER)
	assert.Nil(t, err, "RPC client should be created properly")
	var wg sync.WaitGroup
	for n := 1; n <= N; n++ {
		nn := int(math.Pow(10, float64(n)))
		tArr := make([]int64, nn)
		t0 := time.Now()
		for i := 0; i < nn; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				arg := echo.EchoRequest{Text: "Hello!"}
				res := echo.EchoResult{}
				t1 := time.Now()
				err = pdc.RPC("Echo.Echo", &arg, &res)
				tArr[i] = time.Since(t1).Microseconds()
				assert.Equal(t, "Hello!", res.Text)
				assert.Nil(t, err, "RPC call should succeed")
			}(i)
		}
		wg.Wait()
		totalTime := time.Since(t0).Microseconds()
		sum := int64(0)
		for _, t := range tArr {
			sum += t
		}
		dbg.DPrintf(dbg.TEST, "Request Number: %v; Total time: %v; Avg Lat: %v", nn, totalTime, sum/int64(nn))
	}

	// Stop server
	assert.Nil(t, tse.Stop())
}
