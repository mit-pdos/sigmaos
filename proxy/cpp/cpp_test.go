package cpp_test

import (
	"flag"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"net"
	rpcclnt "sigmaos/rpc/clnt"
	"sigmaos/rpc/clnt/channel/rpcchannel"
	rpcclntopts "sigmaos/rpc/clnt/opts"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	echoproto "sigmaos/example/example_echo_server/proto"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var prewarm bool = false

func init() {
	flag.BoolVar(&prewarm, "prewarm", false, "Pre-warm the CPP proc")
}

func runSpawnLatency(ts *test.RealmTstate, kernels []string, evict bool, ncore proc.Tmcpu) *proc.Proc {
	args := []string{}
	if evict {
		args = append(args, "waitEvict")
	}
	p := proc.NewProc("spawn-latency-cpp", args)
	p.SetMcpu(ncore)
	start1 := time.Now()
	err := ts.Spawn(p)
	assert.Nil(ts.Ts.T, err, "Spawn")
	err = ts.WaitStart(p.GetPid())
	assert.Nil(ts.Ts.T, err, "Start")
	SLEEP := 2 * time.Second
	start := time.Now()
	var evicted bool
	if evict {
		go func() {
			time.Sleep(SLEEP)
			evicted = true
			err := ts.Evict(p.GetPid())
			assert.Nil(ts.Ts.T, err, "Evict")
		}()
	}
	db.DPrintf(db.TEST, "CPP proc started (lat=%v)", time.Since(start1))
	status, err := ts.WaitExit(p.GetPid())
	db.DPrintf(db.TEST, "CPP proc exited, status: %v", status)
	assert.Nil(ts.Ts.T, err, "WaitExit error")
	if evict {
		assert.True(ts.Ts.T, evicted && time.Since(start) >= SLEEP, "Exited too fast %v %v", evicted, time.Since(start))
		assert.True(ts.Ts.T, status != nil && status.IsStatusEvicted(), "Exit status wrong: %v", status)
	} else {
		assert.True(ts.Ts.T, status != nil && status.IsStatusOK(), "Exit status wrong: %v", status)
	}
	return p
}

func TestCompile(t *testing.T) {
}

func TestSpawnWaitExit(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	db.DPrintf(db.TEST, "Running proc")
	p := runSpawnLatency(mrts.GetRealm(test.REALM1), nil, false, 0)
	db.DPrintf(db.TEST, "Proc done")

	b, err := mrts.GetRealm(test.REALM1).GetFile(filepath.Join(sp.S3, sp.LOCAL, "9ps3/hello-cpp-1"))
	assert.Nil(mrts.T, err, "Err GetFile: %v", err)
	assert.True(mrts.T, strings.Contains(string(b), p.GetPid().String()), "Proc output not in file")
}

func TestSpawnWaitEvict(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	db.DPrintf(db.TEST, "Running proc")
	runSpawnLatency(mrts.GetRealm(test.REALM1), nil, true, 0)
	db.DPrintf(db.TEST, "Proc done")
}

func TestSpawnLatency(t *testing.T) {
	const (
		N_PROC = 15
		N_NODE = 8
	)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	if err := mrts.GetRealm(test.REALM1).BootNode(N_NODE); !assert.Nil(t, err, "Err boot: %v", err) {
		return
	}

	runSpawnLatency(mrts.GetRealm(test.REALM1), nil, false, 2000)

	db.DPrintf(db.TEST, "Running procs")
	c := make(chan bool)
	for i := 0; i < N_PROC; i++ {
		go func(c chan bool) {
			runSpawnLatency(mrts.GetRealm(test.REALM1), nil, false, 2000)
			c <- true
		}(c)
	}
	for i := 0; i < N_PROC; i++ {
		<-c
	}
	db.DPrintf(db.TEST, "Procs done")
}

func TestEchoServerProc(t *testing.T) {
	const (
		SERVER_PROC_MCPU proc.Tmcpu = 1000
		SRV_PN           string     = "name/echo-srv-cpp"
	)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	p := proc.NewProc("echo-srv-cpp", nil)
	p.SetMcpu(SERVER_PROC_MCPU)
	db.DPrintf(db.TEST, "Spawn server proc %v", p)
	start := time.Now()
	err := mrts.GetRealm(test.REALM1).Spawn(p)
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Spawn")
	err = mrts.GetRealm(test.REALM1).WaitStart(p.GetPid())
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Start")
	db.DPrintf(db.TEST, "CPP server proc started (lat=%v)", time.Since(start))

	// Verify the endpoint has been correctly posted
	ep, err := mrts.GetRealm(test.REALM1).ReadEndpoint(SRV_PN)
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "ReadEndpoint: %v", err)
	db.DPrintf(db.TEST, "CPP Echo srv EP: %v", ep)

	conn, err := net.Dial("tcp", ep.Addrs()[0].IPPort())
	if assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Dial: %v", err) {
		ch := rpcchannel.NewRPCChannel(conn)
		rpcc, err := rpcclnt.NewRPCClnt("echosrv", rpcclntopts.WithRPCChannel(ch))
		if assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "New RPCClnt") {
			arg := &echoproto.EchoReq{
				Text: "Hello there!",
				Num1: 1,
				Num2: 2,
			}
			var res echoproto.EchoRep
			err = rpcc.RPC("EchoSrv.Echo", arg, &res)
			assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Error echo RPC: %v", err)
			assert.Equal(mrts.T, arg.Text, res.Text, "Didn't echo correctly")
			assert.Equal(mrts.T, arg.Num1+arg.Num2, res.Res, "Didn't add correctly: %v + %v != %v", arg.Num1, arg.Num2, res.Res)
			err = rpcc.RPC("EchoSrv.Echo234", arg, &res)
			assert.NotNil(mrts.GetRealm(test.REALM1).Ts.T, err, "Unexpectedly succeeded unknown RPC: %v", err)
		}
	}

	err = mrts.GetRealm(test.REALM1).Evict(p.GetPid())
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Evict")

	status, err := mrts.GetRealm(test.REALM1).WaitExit(p.GetPid())
	db.DPrintf(db.TEST, "CPP proc exited, status: %v", status)
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "WaitExit error")
	assert.True(mrts.GetRealm(test.REALM1).Ts.T, status != nil && status.IsStatusEvicted(), "Exit status wrong: %v", status)
	db.DPrintf(db.TEST, "Proc done")
}
