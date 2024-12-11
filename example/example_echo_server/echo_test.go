package example_echo_server_test

import (
	"math"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	echo "sigmaos/example/example_echo_server"
	"sigmaos/proc"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/rand"
)

type TstateEcho struct {
	*test.Tstate
	jobname string
	pid     sp.Tpid
}

func newTstateEcho(t *testing.T) (*TstateEcho, error) {
	jobname := rand.Name()
	jobdir := filepath.Join(echo.DIR_ECHO_SERVER, jobname)
	var err error
	tse := &TstateEcho{}
	tse.jobname = jobname
	ts, err := test.NewTstateAll(t)
	if err != nil {
		db.DPrintf(db.ERROR, "Error New Tstate: %v", err)
		return nil, err
	}
	tse.Tstate = ts
	tse.MkDir(echo.DIR_ECHO_SERVER, 0777)
	if err := tse.MkDir(jobdir, 0777); err != nil {
		db.DPrintf(db.ERROR, "Error mkdir: %v", err)
		tse.Shutdown()
		return nil, err
	}
	// Start proc
	p := proc.NewProc("example-echo", []string{})
	p.SetMcpu(proc.Tmcpu(1000))
	if err := tse.Spawn(p); !assert.Nil(t, err, "Err spawn: %v", err) {
		tse.Shutdown()
		return nil, err
	}
	if err = tse.WaitStart(p.GetPid()); !assert.Nil(t, err, "Error spawn proc: %v", nil) {
		tse.Shutdown()
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

func TestCompile(t *testing.T) {
}

func TestEcho(t *testing.T) {
	// start server
	tse, err := newTstateEcho(t)
	if !assert.Nil(t, err, "Test server should start properly %v", err) {
		return
	}

	// create a RPC client and query server
	rpcc, err := sprpcclnt.NewRPCClnt(tse.FsLib, echo.NAMED_ECHO_SERVER)
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
	if !assert.Nil(t, err, "Test server should start properly %v", err) {
		return
	}

	// create a RPC client and query server
	rpcc, err := sprpcclnt.NewRPCClnt(tse.FsLib, echo.NAMED_ECHO_SERVER)
	assert.Nil(t, err, "RPC client should be created properly")
	arg := echo.EchoRequest{Text: "Hello World!"}
	res := echo.EchoResult{}
	N_REQ := 10000
	t0 := time.Now()
	for i := 0; i < N_REQ; i++ {
		rpcc.RPC("EchoSrv.Echo", &arg, &res)
	}
	totalTime := time.Since(t0).Microseconds()
	db.DPrintf(db.ALWAYS, "Total time: %v ms; avg time %v ms", totalTime, totalTime/int64(N_REQ))

	// Stop server
	assert.Nil(t, tse.Stop())
}

func TestEchoLoad(t *testing.T) {
	const (
		N              = 3
		N_RPC_SESSIONS = 1
	)
	// start server
	tse, err := newTstateEcho(t)
	if !assert.Nil(t, err, "Test server should start properly %v", err) {
		return
	}

	// create a RPC client and query server
	fsls := make([]*fslib.FsLib, 0, N_RPC_SESSIONS)
	for i := 0; i < N_RPC_SESSIONS; i++ {
		pe := proc.NewAddedProcEnv(tse.ProcEnv())
		sc, err := tse.NewClnt(0, pe)
		fsl := sc.FsLib
		if !assert.Nil(t, err, "Error newfsl: %v", nil) {
			db.DPrintf(db.ERROR, "Error newfsl: %v", err)
		}
		fsls = append(fsls, fsl)
	}
	rpcc, err := sprpcclnt.NewRPCClnt(tse.FsLib, echo.NAMED_ECHO_SERVER)
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
				err = rpcc.RPC("EchoSrv.Echo", &arg, &res)
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
		db.DPrintf(db.TEST, "Request Number: %v; Total time: %v; Avg Lat: %v", nn, totalTime, sum/int64(nn))
	}

	// Stop server
	assert.Nil(t, tse.Stop())
}
