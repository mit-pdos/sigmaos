package netperf_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/netperf"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestCompile(t *testing.T) {
}

func clntDialDialProxy(t *testing.T, ep *sp.Tendpoint) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	npc := ts.GetDialProxyClnt()
	_, err := netperf.ClntDialDialProxy(ntrial, npc, ep)
	assert.Nil(ts.T, err, "Err clnt: %v", err)
}

func srvDialDialProxy(t *testing.T, addr *sp.Taddr, epType sp.TTendpoint) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	npc := ts.GetDialProxyClnt()
	started := make(chan bool, 2)
	err := netperf.SrvDialDialProxy(started, ntrial, npc, addr, epType)
	assert.Nil(ts.T, err, "Err srv: %v", err)
}

// Measure performance of dialing using the DialProxy using procs (one server
// proc and one client proc).
func TestProcDialDialProxy(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	// Start srv proc
	srvProc := proc.NewProc("netperf-srv", []string{"dial", srvaddr, strconv.Itoa(ntrial)})
	err := ts.Spawn(srvProc)
	assert.Nil(ts.T, err, "Spawn srv proc: %v", err)
	err = ts.WaitStart(srvProc.GetPid())
	assert.Nil(ts.T, err, "WaitStart srv proc: %v", err)

	// Start clnt proc
	clntProc := proc.NewProc("netperf-clnt", []string{"dial", srvaddr, strconv.Itoa(ntrial)})
	err = ts.Spawn(clntProc)
	assert.Nil(ts.T, err, "Spawn clnt proc: %v", err)

	// Wait for both to exit
	clntStatus, err := ts.WaitExit(clntProc.GetPid())
	if assert.Nil(ts.T, err, "WaitExit clnt proc: %v", err) {
		assert.True(ts.T, clntStatus.IsStatusOK(), "Err clnt status: %v", clntStatus)
	}
	srvStatus, err := ts.WaitExit(srvProc.GetPid())
	if assert.Nil(ts.T, err, "WaitExit srv proc: %v", err) {
		assert.True(ts.T, srvStatus.IsStatusOK(), "Err srv status: %v", srvStatus)
	}
	db.DPrintf(db.BENCH, "Clnt latency: %s", clntStatus.Msg())
}

// Client-side stub to measure performance of dialing using the DialProxy for
// internal EPs. Useful for running cross-machine performance benchmarks.
func TestClntDialDialProxyInternal(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	ep := sp.NewEndpoint(sp.INTERNAL_EP, sp.Taddrs{addr})
	clntDialDialProxy(t, ep)
}

// Server-side stub to measure performance of dialing using the DialProxy for
// internal EPs. Useful for running cross-machine performance benchmarks.
func TestSrvDialDialProxyInternal(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	srvDialDialProxy(t, addr, sp.INTERNAL_EP)
}

// Client-side stub to measure performance of dialing using the DialProxy for
// external EPs. Useful for running cross-machine performance benchmarks.
func TestClntDialDialProxyExternal(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	ep := sp.NewEndpoint(sp.EXTERNAL_EP, sp.Taddrs{addr})
	clntDialDialProxy(t, ep)
}

// Server-side stub to measure performance of dialing using the DialProxy for
// external EPs. Useful for running cross-machine performance benchmarks.
func TestSrvDialDialProxyExternal(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	srvDialDialProxy(t, addr, sp.EXTERNAL_EP)
}

// Client-side stub to measure network throughput using the DialProxy.
// Useful for running cross-machine performance benchmarks.
func TestClntThroughputDialProxy(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	ep := sp.NewEndpoint(sp.INTERNAL_EP, sp.Taddrs{addr})

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	npc := ts.GetDialProxyClnt()
	db.DPrintf(db.TEST, "Client start")
	conn, err := npc.Dial(ep)
	assert.Nil(ts.T, err, "Err Dial: %v", err)
	clntThroughput(t, conn)
}

// Server-side stub to measure network throughput using the DialProxy.
// Useful for running cross-machine performance benchmarks.
func TestSrvThroughputDialProxy(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	npc := ts.GetDialProxyClnt()
	_, l, err := npc.Listen(sp.INTERNAL_EP, addr)
	assert.Nil(ts.T, err, "Err Listen: %v", err)
	db.DPrintf(db.TEST, "Ready to accept connections")
	conn, err := l.Accept()
	assert.Nil(ts.T, err, "Err Accept: %v", err)
	srvThroughput(t, conn)
}

// Client-side stub to measure network TCP RTT using the DialProxy.
// Useful for running cross-machine performance benchmarks.
func TestClntRTTDialProxy(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	ep := sp.NewEndpoint(sp.INTERNAL_EP, sp.Taddrs{addr})

	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	npc := ts.GetDialProxyClnt()
	db.DPrintf(db.TEST, "Client start")
	conn, err := npc.Dial(ep)
	assert.Nil(ts.T, err, "Err Dial: %v", err)
	clntRTT(t, conn)
}

// Server-side stub to measure network TCP RTT using the DialProxy.
// Useful for running cross-machine performance benchmarks.
func TestSrvRTTDialProxy(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	npc := ts.GetDialProxyClnt()
	_, l, err := npc.Listen(sp.INTERNAL_EP, addr)
	assert.Nil(ts.T, err, "Err Listen: %v", err)
	db.DPrintf(db.TEST, "Ready to accept connections")
	conn, err := l.Accept()
	assert.Nil(ts.T, err, "Err Accept: %v", err)
	srvRTT(t, conn)
}
