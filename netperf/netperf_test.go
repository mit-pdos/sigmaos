package netperf_test

import (
	"flag"
	"net"
	"testing"
	"time"

	"github.com/montanaflynn/stats"
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var srvaddr string
var ntrial int

func init() {
	flag.StringVar(&srvaddr, "srvaddr", ":8080", "Address of server.")
	flag.IntVar(&ntrial, "ntrial", 50, "Number of trials.")
}

func TestCompile(t *testing.T) {
}

func clntDialTCP(t *testing.T, addr *sp.Taddr) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	db.DPrintf(db.TEST, "Client start dialing")
	lat := make([]float64, 0, ntrial)
	for i := 0; i < ntrial; i++ {
		start := time.Now()
		// Dial the listener
		conn, err := net.Dial("tcp", addr.IPPort())
		assert.Nil(ts.T, err, "Err Dial: %v", err)
		lat = append(lat, float64(time.Since(start).Microseconds()))
		err = conn.Close()
		assert.Nil(ts.T, err, "Err Close: %v", err)
		time.Sleep(50 * time.Millisecond)
	}
	avgLat, err := stats.Mean(lat)
	assert.Nil(ts.T, err, "Err Mean: %v", err)
	stdLat, err := stats.StandardDeviation(lat)
	assert.Nil(ts.T, err, "Err Std: %v", err)
	db.DPrintf(db.BENCH, "Raw latency: %vus", lat)
	db.DPrintf(db.BENCH, "Mean latency: %vus", avgLat)
	db.DPrintf(db.BENCH, "Std latency: %vus", stdLat)
}

func srvDialTCP(t *testing.T, addr *sp.Taddr) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	l, err := net.Listen("tcp", addr.IPPort())
	assert.Nil(ts.T, err, "Err Listen: %v", err)
	db.DPrintf(db.TEST, "Ready to accept connections")
	for i := 0; i < ntrial; i++ {
		conn, err := l.Accept()
		assert.Nil(ts.T, err, "Err Accept: %v", err)
		err = conn.Close()
		assert.Nil(ts.T, err, "Err Close: %v", err)
	}
	db.DPrintf(db.TEST, "Done accepting connections")
}

func clntDialNetProxy(t *testing.T, ep *sp.Tendpoint) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	npc := ts.GetNetProxyClnt()
	db.DPrintf(db.TEST, "Client start dialing")
	lat := make([]float64, 0, ntrial)
	for i := 0; i < ntrial; i++ {
		start := time.Now()
		// Dial the listener
		conn, err := npc.Dial(ep)
		assert.Nil(ts.T, err, "Err Dial: %v", err)
		lat = append(lat, float64(time.Since(start).Microseconds()))
		err = conn.Close()
		assert.Nil(ts.T, err, "Err Close: %v", err)
		time.Sleep(50 * time.Millisecond)
	}
	avgLat, err := stats.Mean(lat)
	assert.Nil(ts.T, err, "Err Mean: %v", err)
	stdLat, err := stats.StandardDeviation(lat)
	assert.Nil(ts.T, err, "Err Std: %v", err)
	db.DPrintf(db.BENCH, "Raw latency: %vus", lat)
	db.DPrintf(db.BENCH, "Mean latency: %vus", avgLat)
	db.DPrintf(db.BENCH, "Std latency: %vus", stdLat)
}

func srvDialNetProxy(t *testing.T, addr *sp.Taddr, epType sp.TTendpoint) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	npc := ts.GetNetProxyClnt()
	_, l, err := npc.Listen(epType, addr)
	assert.Nil(ts.T, err, "Err Listen: %v", err)
	db.DPrintf(db.TEST, "Ready to accept connections")
	for i := 0; i < ntrial; i++ {
		conn, err := l.Accept()
		assert.Nil(ts.T, err, "Err Accept: %v", err)
		err = conn.Close()
		assert.Nil(ts.T, err, "Err Close: %v", err)
	}
	db.DPrintf(db.TEST, "Done accepting connections")
}

func TestClntDialNetProxyInternal(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	ep := sp.NewEndpoint(sp.INTERNAL_EP, sp.Taddrs{addr})
	clntDialNetProxy(t, ep)
}

func TestSrvDialNetProxyInternal(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	srvDialNetProxy(t, addr, sp.INTERNAL_EP)
}

func TestClntDialNetProxyExternal(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	ep := sp.NewEndpoint(sp.EXTERNAL_EP, sp.Taddrs{addr})
	clntDialNetProxy(t, ep)
}

func TestSrvDialNetProxyExternal(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	srvDialNetProxy(t, addr, sp.EXTERNAL_EP)
}

func TestClntDialTCP(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	clntDialTCP(t, addr)
}

func TestSrvDialTCP(t *testing.T) {
	addr, err := sp.NewTaddrFromString(srvaddr, sp.INNER_CONTAINER_IP)
	assert.Nil(t, err, "Err parse addr: %v", err)
	srvDialTCP(t, addr)
}
