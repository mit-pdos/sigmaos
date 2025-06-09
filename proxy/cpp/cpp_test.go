package cpp_test

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/cossim"
	cossimproto "sigmaos/apps/cossim/proto"
	cossimsrv "sigmaos/apps/cossim/srv"
	"sigmaos/apps/epcache"
	epcachesrv "sigmaos/apps/epcache/srv"
	spinproto "sigmaos/apps/spin/proto"
	db "sigmaos/debug"
	echoproto "sigmaos/example/example_echo_server/proto"
	"sigmaos/proc"
	rpcncclnt "sigmaos/rpc/clnt/netconn"
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
	p.GetProcEnv().UseSPProxy = true
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
	p.GetProcEnv().UseSPProxy = true
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

	rpcc, err := rpcncclnt.NewTCPRPCClnt("echosrv", ep)
	if !assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "new rpc clnt: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Created echosrv RPC clnt")

	arg := &echoproto.EchoReq{
		Text: "Hello there!",
		Num1: 1,
		Num2: 2,
	}
	var rep echoproto.EchoRep
	db.DPrintf(db.TEST, "Send good EchoSrv.Echo RPC")
	err = rpcc.RPC("EchoSrv.Echo", arg, &rep)
	db.DPrintf(db.TEST, "Recv good EchoSrv.Echo RPC reply")
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Error echo RPC: %v", err)
	assert.Equal(mrts.T, arg.Text, rep.Text, "Didn't echo correctly")
	assert.Equal(mrts.T, arg.Num1+arg.Num2, rep.Res, "Didn't add correctly: %v + %v != %v", arg.Num1, arg.Num2, rep.Res)
	db.DPrintf(db.TEST, "Send bad EchoSrv.Echo RPC")
	err = rpcc.RPC("EchoSrv.Echo234", arg, &rep)
	db.DPrintf(db.TEST, "Recv bad EchoSrv.Echo RPC reply")
	assert.NotNil(mrts.GetRealm(test.REALM1).Ts.T, err, "Unexpectedly succeeded unknown RPC: %v", err)

	db.DPrintf(db.TEST, "Evict echosrv")
	err = mrts.GetRealm(test.REALM1).Evict(p.GetPid())
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Evict")

	db.DPrintf(db.TEST, "WaitExit echosrv")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(p.GetPid())
	db.DPrintf(db.TEST, "CPP proc exited, status: %v", status)
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "WaitExit error")
	assert.True(mrts.GetRealm(test.REALM1).Ts.T, status != nil && status.IsStatusEvicted(), "Exit status wrong: %v", status)
	db.DPrintf(db.TEST, "Proc done")
}

func TestSpinServerProc(t *testing.T) {
	const (
		SERVER_PROC_MCPU proc.Tmcpu = 1000
		SRV_UNION_DIR    string     = "name/spin-srv-cpp"
	)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	// Make union dir
	if err := mrts.GetRealm(test.REALM1).MkDir(SRV_UNION_DIR, 0777); !assert.Nil(mrts.T, err, "Err mkunion") {
		return
	}

	p := proc.NewProc("spin-srv-cpp", nil)
	p.GetProcEnv().UseSPProxy = true
	p.SetMcpu(SERVER_PROC_MCPU)
	db.DPrintf(db.TEST, "Spawn server proc %v", p)
	start := time.Now()
	err := mrts.GetRealm(test.REALM1).Spawn(p)
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Spawn")
	err = mrts.GetRealm(test.REALM1).WaitStart(p.GetPid())
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Start")
	db.DPrintf(db.TEST, "CPP server proc started (lat=%v)", time.Since(start))

	// Verify the endpoint has been correctly posted
	ep, err := mrts.GetRealm(test.REALM1).ReadEndpoint(filepath.Join(SRV_UNION_DIR, p.GetPid().String()))
	if !assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "ReadEndpoint: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "CPP spin srv EP: %v", ep)

	rpcc, err := rpcncclnt.NewTCPRPCClnt("spinsrv", ep)
	if !assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "new rpc clnt: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Created spinsrv RPC clnt")

	arg := &spinproto.SpinReq{
		N: 100000,
	}
	var rep spinproto.SpinRep
	db.DPrintf(db.TEST, "Send good SpinSrv.Spin RPC")
	err = rpcc.RPC("SpinSrv.Spin", arg, &rep)
	db.DPrintf(db.TEST, "Recv good SpinSrv.Spin RPC reply: %v", rep.N)
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Error spin RPC: %v", err)

	db.DPrintf(db.TEST, "Evict spinsrv")
	err = mrts.GetRealm(test.REALM1).Evict(p.GetPid())
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Evict")

	db.DPrintf(db.TEST, "WaitExit spinsrv")
	status, err := mrts.GetRealm(test.REALM1).WaitExit(p.GetPid())
	db.DPrintf(db.TEST, "CPP proc exited, status: %v", status)
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "WaitExit error")
	assert.True(mrts.GetRealm(test.REALM1).Ts.T, status != nil && status.IsStatusEvicted(), "Exit status wrong: %v", status)
	db.DPrintf(db.TEST, "Proc done")
}

func TestSpinServerSpawnLatency(t *testing.T) {
	const (
		N_PROC               = 1
		N_NODE               = 0
		N_PARALLEL           = 1
		MCPU_PER_PROC        = 2000
		SRV_UNION_DIR string = "name/spin-srv-cpp"
		USE_EPCACHE   bool   = true
	)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	// Make union dir
	if err := mrts.GetRealm(test.REALM1).MkDir(SRV_UNION_DIR, 0777); !assert.Nil(mrts.T, err, "Err mkunion") {
		return
	}

	if err := mrts.GetRealm(test.REALM1).BootNode(N_NODE); !assert.Nil(t, err, "Err boot: %v", err) {
		return
	}

	var epcsrvEP *sp.Tendpoint
	var err error
	// Start the epcache server
	if USE_EPCACHE {
		var j *epcachesrv.EPCacheJob
		j, err = epcachesrv.NewEPCacheJob(mrts.GetRealm(test.REALM1).SigmaClnt)
		if !assert.Nil(mrts.T, err, "Err EPCacheJob: %v", err) {
			return
		}
		epcsrvEP, err = j.GetSrvEP()
		if !assert.Nil(mrts.T, err, "Err Get EPCacheSrv EP: %v", err) {
			return
		}
	}

	p := proc.NewProc("spin-srv-cpp", []string{strconv.FormatBool(USE_EPCACHE)})
	p.GetProcEnv().UseSPProxy = true
	p.SetMcpu(MCPU_PER_PROC)
	if USE_EPCACHE {
		p.SetCachedEndpoint(epcache.EPCACHE, epcsrvEP)
	}
	start := time.Now()
	err = mrts.GetRealm(test.REALM1).Spawn(p)
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Spawn")
	err = mrts.GetRealm(test.REALM1).WaitStart(p.GetPid())
	assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Start")
	db.DPrintf(db.TEST, "CPP server proc started (lat=%v)", time.Since(start))

	db.DPrintf(db.TEST, "Running procs")
	parallelCh := make(chan bool, N_PARALLEL)
	for i := 0; i < N_PARALLEL; i++ {
		parallelCh <- true
	}
	c := make(chan bool)
	for i := 0; i < N_PROC; i++ {
		go func(c chan bool, parallelCh chan bool) {
			<-parallelCh
			p := proc.NewProc("spin-srv-cpp", []string{strconv.FormatBool(USE_EPCACHE)})
			p.GetProcEnv().UseSPProxy = true
			p.SetMcpu(MCPU_PER_PROC)
			if USE_EPCACHE {
				p.SetCachedEndpoint(epcache.EPCACHE, epcsrvEP)
			}
			start := time.Now()
			err := mrts.GetRealm(test.REALM1).Spawn(p)
			assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Spawn")
			err = mrts.GetRealm(test.REALM1).WaitStart(p.GetPid())
			assert.Nil(mrts.GetRealm(test.REALM1).Ts.T, err, "Start")
			db.DPrintf(db.TEST, "Spin server proc started (lat=%v)", time.Since(start))
			parallelCh <- true
			c <- true
		}(c, parallelCh)
	}
	for i := 0; i < N_PROC; i++ {
		<-c
	}
	db.DPrintf(db.TEST, "Procs done")
}

func TestSpinServerExec(t *testing.T) {
	const (
		N_TRIALS = 10
	)
	for i := 0; i < N_TRIALS; i++ {
		p := proc.NewProc("spin-srv-cpp", nil)
		p.GetProcEnv().UseSPProxy = true
		p.SetSpawnTime(time.Now())
		homedir, err := os.UserHomeDir()
		if !assert.Nil(t, err, "Err homedir: %v", err) {
			return
		}
		p.FinalizeEnv("xxx", "yyy", "zzz")
		cmd := exec.Command(filepath.Join(homedir, "sigmaos/bin/user", p.GetVersionedProgram()))
		cmd.Env = append(cmd.Env, p.GetEnv()...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); !assert.Nil(t, err, "Err command start: %v", err) {
			return
		}
		cmd.Wait()
	}
}

func TestCosSimInitLatency(t *testing.T) {
	const (
		N_PROC       = 1
		N_NODE       = 0
		N_PARALLEL   = 1
		MCPU_PER_SRV = 2000
		// App parameters
		N_VEC      = 100
		VEC_DIM    = 100
		EAGER_INIT = false
		// Cache parameters
		N_CACHE    = 1
		CACHE_MCPU = 1000
		CACHE_GC   = true
		JOB_NAME   = "cossim-job"
	)

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	if err := mrts.GetRealm(test.REALM1).BootNode(N_NODE); !assert.Nil(t, err, "Err boot: %v", err) {
		return
	}

	// Start a cossim job
	j, err := cossimsrv.NewCosSimJob(mrts.GetRealm(test.REALM1).SigmaClnt, JOB_NAME, N_VEC, VEC_DIM, EAGER_INIT, MCPU_PER_SRV, N_CACHE, CACHE_MCPU, CACHE_GC)
	if !assert.Nil(mrts.T, err, "Err NewCosSimJob: %v", err) {
		return
	}
	defer j.Stop()

	// Construct input vec
	v := cossim.VectorToSlice(cossim.NewVectors(1, VEC_DIM)[0])
	_, csclnt, startLat, err := j.AddSrv()
	if !assert.Nil(mrts.T, err, "Err AddSrv: %v", err) {
		return
	}
	ranges := []*cossimproto.VecRange{&cossimproto.VecRange{
		StartID: 0,
		EndID:   N_VEC - 1,
	},
	}
	db.DPrintf(db.TEST, "CosSim server proc started (lat=%v)", startLat)
	start := time.Now()
	id, val, err := csclnt.CosSim(v, ranges)
	if !assert.Nil(t, err, "Err CosSim: %v", err) {
		return
	}
	db.DPrintf(db.TEST, "CosSim result: %v %v", id, val)
	db.DPrintf(db.TEST, "CosSim proc handled first request (lat=%v)", time.Since(start))

	db.DPrintf(db.TEST, "Running procs")
	parallelCh := make(chan bool, N_PARALLEL)
	for i := 0; i < N_PARALLEL; i++ {
		parallelCh <- true
	}
	c := make(chan bool)
	for i := 0; i < N_PROC; i++ {
		go func(c chan bool, parallelCh chan bool) {
			<-parallelCh
			start := time.Now()
			_, csclnt, startLat, err := j.AddSrv()
			if !assert.Nil(mrts.T, err, "Err AddSrv: %v", err) {
				return
			}
			db.DPrintf(db.TEST, "CosSim server proc started (lat=%v)", startLat)
			id, val, err := csclnt.CosSim(v, ranges)
			assert.Nil(t, err, "Err CosSim: %v", err)
			db.DPrintf(db.TEST, "CosSim result: %v %v", id, val)
			db.DPrintf(db.TEST, "CosSim proc handled first request (lat=%v)", time.Since(start))
			parallelCh <- true
			c <- true
		}(c, parallelCh)
	}
	for i := 0; i < N_PROC; i++ {
		<-c
	}
	db.DPrintf(db.TEST, "Procs done")
}
