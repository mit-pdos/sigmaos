package provider_test

import (
	// Go imports:

	"log"
	"path"
	"strconv"
	"testing"

	// External imports:
	"github.com/stretchr/testify/assert"

	// SigmaOS imports:

	db "sigmaos/debug"
	echo "sigmaos/example_echo_server"
	"sigmaos/fslib"
	"sigmaos/linuxsched"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestExerciseNamed(t *testing.T) {
	dir := sp.NAMED
	ts := test.NewTstatePath(t, dir)

	sts, err := ts.GetDir(dir)
	assert.Nil(t, err)

	log.Printf("%v: %v\n", dir, sp.Names(sts))

	// Your code here
	fn := dir + "tfile"
	d := []byte("hello")
	_, err = ts.PutFile(fn, 0777, sp.ORDWR, d)
	assert.Equal(t, nil, err)

	sts, err = ts.GetDir(dir)
	assert.Nil(t, err)

	log.Printf("%v: %v\n", dir, sp.Names(sts))

	rdr, err := ts.OpenReader(fn)
	assert.Equal(t, nil, err)

	b, err := rdr.GetData()
	assert.Equal(t, nil, err)
	assert.Equal(t, d, b)

	err = ts.Remove(fn)
	assert.Equal(t, nil, err)

	ts.Shutdown()
}

func TestExerciseProc(t *testing.T) {
	ts := test.NewTstateAll(t)

	p := proc.NewProc("example", []string{})
	err := ts.Spawn(p)
	assert.Nil(t, err)
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	// Once you modified cmd/user/example, you should
	// pass this test:
	assert.Equal(t, "Hello world", status.Msg())

	ts.Shutdown()
}

func TestExerciseProcWithProvider(t *testing.T) {
	ts := test.NewTstateAllWithProvider(t, sp.T_CLOUDLAB)

	p := proc.NewProc("example", []string{})
	p.SetProvider(sp.T_AWS)
	err := ts.Spawn(p)
	assert.Nil(t, err)
	err = ts.WaitStart(p.GetPid())
	assert.NotNil(t, err)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, status)
	assert.NotNil(t, err)

	ts.Shutdown()
}

func TestExerciseProcWithProviderNotAvailable(t *testing.T) {
	ts := test.NewTstateAllWithProvider(t, sp.T_CLOUDLAB)

	p := proc.NewProc("example", []string{})
	p.SetProvider(sp.T_AWS)
	err := ts.Spawn(p)
	assert.Nil(t, err)
	err = ts.WaitStart(p.GetPid())
	assert.NotNil(t, err)
	status, err := ts.WaitExit(p.GetPid())
	assert.Nil(t, status)
	assert.NotNil(t, err)

	ts.Shutdown()
}

func checkNumSchedds(t *testing.T, ts *test.Tstate, targetNum int) {
	scheddSts, err := ts.GetDir(sp.SCHEDD)
	assert.Nil(t, err, "Err getting schedd dir: %v", err)
	scheddNames := sp.Names(scheddSts)
	log.Printf("%v: %v\n", sp.SCHEDD, scheddNames)
	assert.Equal(t, len(scheddNames), targetNum, "Err: incorrect number of schedds (wanted %v, got %v)", len(scheddNames), targetNum)
}

func checkNumLcscheds(t *testing.T, ts *test.Tstate, targetNum int) {
	numLcscheds := 0
	for _, prvdr := range []sp.Tprovider{sp.T_AWS, sp.T_CLOUDLAB} {
		lcscheddir := sp.LCSCHED + prvdr.TproviderToDir()
		lcschedsts, err := ts.GetDir(lcscheddir)
		assert.Nil(t, err)
		lcschedNames := sp.Names(lcschedsts)
		log.Printf("%v: %v\n", lcscheddir, lcschedNames)
		numLcscheds += len(lcschedNames)
	}

	assert.Equal(t, numLcscheds, targetNum, "Err: incorrect number of lcscheds (wanted %v, got %v)", targetNum, numLcscheds)
}

func checkNumProcqs(t *testing.T, ts *test.Tstate, targetNum int) {
	numProcqs := 0
	for _, prvdr := range []sp.Tprovider{sp.T_AWS, sp.T_CLOUDLAB} {
		procqdir := sp.PROCQ + prvdr.TproviderToDir()
		procqsts, err := ts.GetDir(procqdir)
		assert.Nil(t, err)
		procqNames := sp.Names(procqsts)
		log.Printf("%v: %v\n", procqdir, procqNames)
		numProcqs += len(procqNames)
	}

	assert.Equal(t, numProcqs, targetNum, "Err: incorrect number of procqs (wanted %v, got %v)", targetNum, numProcqs)
}

func spawnMultiProviderNodes(t *testing.T, providersToNumNodes [][]interface{}) (*test.Tstate, int) {
	var err error

	provider1 := providersToNumNodes[0][0].(sp.Tprovider)
	db.DPrintf(db.TEST, "Boot first node with provider %v", provider1)
	ts := test.NewTstateAllWithProvider(t, provider1)

	numExtraProvider1Nodes := providersToNumNodes[0][1].(int) - 1
	db.DPrintf(db.TEST, "Boot %v extra node(s) with provider %v", numExtraProvider1Nodes, provider1)
	err = ts.BootNodeWithProvider(numExtraProvider1Nodes, provider1)
	assert.Nil(t, err, "Err boot node: %v", err)

	numNodesTotal := 1 + numExtraProvider1Nodes
	var provider sp.Tprovider
	var numNodes int
	for _, otherProviders := range providersToNumNodes[1:] {
		provider = otherProviders[0].(sp.Tprovider)
		numNodes = otherProviders[1].(int)

		// Need to boot 1 lcschednode per provider
		db.DPrintf(db.TEST, "Boot lcsched node with provider %v", provider)
		err = ts.BootLcschedNodeWithProvider(provider)
		assert.Nil(t, err, "Err boot lcsched node: %v", err)

		// After booting 1 lcschednode for the provider, boot the rest as normal nodes
		db.DPrintf(db.TEST, "Boot %v extra node(s) with provider %v", numNodes-1, provider)
		err = ts.BootNodeWithProvider(numNodes-1, provider)
		assert.Nil(t, err, "Err boot node: %v", err)

		numNodesTotal += numNodes
	}

	return ts, numNodesTotal
}

func TestExerciseBootMultiProvidersBase(t *testing.T) {
	var err error
	providers := sp.AllProviders()
	numProviders := len(providers)

	provider1 := providers[0]
	db.DPrintf(db.TEST, "Boot first node with provider %v", provider1)
	ts := test.NewTstateAllWithProvider(t, provider1)

	for _, provider := range providers[1:] {
		db.DPrintf(db.TEST, "Boot lcsched node with provider %v", provider)
		err = ts.BootLcschedNodeWithProvider(provider)
		assert.Nil(t, err, "Err boot lcsched node: %v", err)
	}

	checkNumSchedds(t, ts, numProviders)
	checkNumLcscheds(t, ts, numProviders)
	checkNumProcqs(t, ts, numProviders)

	ts.Shutdown()
}

func TestExerciseBootMultiProvidersExtraNodes(t *testing.T) {
	providersToNumNodes := [][]interface{}{
		{sp.T_CLOUDLAB, 5},
		{sp.T_AWS, 3},
	}

	ts, numNodesTotal := spawnMultiProviderNodes(t, providersToNumNodes)

	checkNumSchedds(t, ts, numNodesTotal)
	checkNumLcscheds(t, ts, len(providersToNumNodes)) // should be 1 lcschednode per provider
	checkNumProcqs(t, ts, numNodesTotal)

	ts.Shutdown()
}

func TestExerciseBootMultiProvidersWithProcs(t *testing.T) {
	providersToNumNodes := [][]interface{}{
		{sp.T_CLOUDLAB, 3},
		{sp.T_AWS, 2},
	}
	var err error

	ts, _ := spawnMultiProviderNodes(t, providersToNumNodes)

	jobname := rand.String(8)
	jobdir := path.Join(echo.DIR_ECHO_SERVER, jobname)
	ts.MkDir(echo.DIR_ECHO_SERVER, 0777)
	err = ts.MkDir(jobdir, 0777)
	assert.Nil(t, err)

	p_echo := proc.NewProc("example-echo", []string{strconv.FormatBool(test.Overlays)})
	p_echo.SetProvider(providersToNumNodes[0][0].(sp.Tprovider))

	p_ex := proc.NewProc("example", []string{})
	p_ex.SetProvider(providersToNumNodes[1][0].(sp.Tprovider))

	ts.Spawn(p_echo)
	ts.Spawn(p_ex)

	if err = ts.WaitStart(p_echo.GetPid()); err != nil {
		db.DFatalf("Error spawn proc %v: %v", p_echo, err)
	}

	if err = ts.WaitStart(p_ex.GetPid()); err != nil {
		db.DFatalf("Error spawn proc %v: %v", p_ex, err)
	}

	// create a RPC client and query server
	rpcc, err := rpcclnt.NewRPCClnt([]*fslib.FsLib{ts.FsLib}, echo.NAMED_ECHO_SERVER)
	assert.Nil(t, err, "RPC client should be created properly")
	arg := echo.EchoRequest{Text: "Hello World!"}
	res := echo.EchoResult{}
	err = rpcc.RPC("EchoSrv.Echo", &arg, &res)
	assert.Nil(t, err, "RPC call should succeed")
	log.Printf("echo result: %v", res.Text)
	assert.Equal(t, "Hello World!", res.Text)

	status, err := ts.WaitExit(p_ex.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK())

	// Once you modified cmd/user/example, you should
	// pass this test:
	assert.Equal(t, "Hello world", status.Msg())

	procs, _ := ts.GetDir("name/example")
	log.Printf("%v: %v\n", "name/example", sp.Names(procs))

	ts.Shutdown()
}

func burstSpawnSpinner(t *testing.T, ts *test.Tstate, N uint, provider sp.Tprovider) []*proc.Proc {
	ps := make([]*proc.Proc, 0, N)
	for i := uint(0); i < N; i++ {
		p := proc.NewProc("spinner", []string{"name/"})
		p.SetProvider(provider)
		p.SetMcpu(1000)
		err := ts.Spawn(p)
		assert.Nil(t, err, "Failed spawning some procs: %v", err)
		ps = append(ps, p)
	}
	return ps
}

func TestSpawnSpinner(t *testing.T) {
	// Boot initial kernel with initial provider and lschednode for other provider(s)
	ts := test.NewTstateAllWithProvider(t, sp.T_AWS)
	ts.BootLcschedNodeWithProvider(sp.T_CLOUDLAB)

	// Number of spinners to burst-spawn
	N := (linuxsched.GetNCores()) * 4
	log.Printf("N: %v", N)

	// Start a couple new procds.
	err := ts.BootNodeWithProvider(1, sp.T_AWS)
	assert.Nil(t, err, "BootNode %v", err)
	err = ts.BootNodeWithProvider(1, sp.T_CLOUDLAB)
	assert.Nil(t, err, "BootNode %v", err)

	db.DPrintf(db.TEST, "Start burst spawn")

	ps1 := burstSpawnSpinner(t, ts, N/2, sp.T_AWS)
	ps2 := burstSpawnSpinner(t, ts, N/2, sp.T_CLOUDLAB)

	ps := append(ps1, ps2...)

	for _, p := range ps {
		err := ts.WaitStart(p.GetPid())
		assert.Nil(t, err, "WaitStart: %v", err)
	}

	for _, p := range ps {
		err := ts.Evict(p.GetPid())
		assert.Nil(t, err, "Evict: %v", err)
	}

	for _, p := range ps {
		status, err := ts.WaitExit(p.GetPid())
		assert.Nil(t, err, "WaitExit: %v", err)
		assert.True(t, status.IsStatusEvicted(), "Wrong status: %v", status)
	}

	ts.Shutdown()
}

func cleanSleeperResult(t *testing.T, ts *test.Tstate, pid sp.Tpid) {
	ts.Remove("name/" + pid.String() + "_out")
}

func TestSpawnManyProcsParallel(t *testing.T) {
	// Boot initial kernel with initial provider and lschednode for other provider(s)
	ts := test.NewTstateAllWithProvider(t, sp.T_AWS)
	ts.BootLcschedNodeWithProvider(sp.T_CLOUDLAB)

	const N_CONCUR = 5   // 13
	const N_SPAWNS = 100 // 500

	err := ts.BootNodeWithProvider(5, sp.T_AWS)
	assert.Nil(t, err, "BootProcd 1")

	err = ts.BootNodeWithProvider(5, sp.T_CLOUDLAB)
	assert.Nil(t, err, "BootProcd 2")

	done := make(chan int)

	for i := 0; i < N_CONCUR; i++ {
		go func(i int) {
			for j := 0; j < N_SPAWNS; j++ {
				pid := sp.GenPid("sleeper")
				db.DPrintf(db.TEST, "Prep spawn %v", pid)
				a := proc.NewProcPid(pid, "sleeper", []string{"0ms", "name/"})
				err := ts.Spawn(a)
				assert.Nil(t, err, "Spawn err %v", err)
				db.DPrintf(db.TEST, "Done spawn %v", pid)

				db.DPrintf(db.TEST, "Prep WaitStart %v", pid)
				err = ts.WaitStart(a.GetPid())
				db.DPrintf(db.TEST, "Done WaitStart %v", pid)
				assert.Nil(t, err, "WaitStart error")

				db.DPrintf(db.TEST, "Prep WaitExit %v", pid)
				status, err := ts.WaitExit(a.GetPid())
				db.DPrintf(db.TEST, "Done WaitExit %v", pid)
				assert.Nil(t, err, "WaitExit")
				assert.True(t, status.IsStatusOK(), "Status not OK")
				cleanSleeperResult(t, ts, pid)
			}
			done <- i
		}(i)
	}
	for i := 0; i < N_CONCUR; i++ {
		x := <-done
		db.DPrintf(db.TEST, "Done %v", x)
	}

	ts.Shutdown()
}
