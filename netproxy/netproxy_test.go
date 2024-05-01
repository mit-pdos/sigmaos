package netproxy_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/netproxy"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	IP       sp.Tip   = sp.NO_IP
	PORT     sp.Tport = 30303
	TEST_MSG          = "hello"
)

func TestCompile(t *testing.T) {
}

func TestBoot(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	db.DPrintf(db.TEST, "Booted")
	ts.Shutdown()
}

// Make sure dialing works (against a normal net.Listener)
func TestDial(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, sp.INNER_CONTAINER_IP, PORT)
	ep := sp.NewEndpoint(sp.Taddrs{addr}, sp.ROOTREALM)
	err := ts.MintAndSetEndpointToken(ep)
	assert.Nil(t, err, "Err Mint EP token: %v", err)
	npc := ts.GetNetProxyClnt()
	c := make(chan bool)
	// Create a listener directly
	l, err := net.Listen("tcp", addr.IPPort())
	assert.Nil(t, err, "Err Listen: %v", err)
	go func(l net.Listener, c chan bool) {
		b := make([]byte, len(TEST_MSG))
		// Accept connections
		conn, err := l.Accept()
		assert.Nil(t, err, "Err accept: %v", err)
		n, err := conn.Read(b)
		assert.Nil(t, err, "Err read: %v", err)
		assert.Equal(t, len(b), n, "Err read nbyte: %v != %v", len(b), n)
		l.Close()
		c <- true
	}(l, c)
	// Dial the listener
	conn, err := npc.Dial(ep)
	assert.Nil(t, err, "Err Dial: %v", err)
	n, err := conn.Write([]byte(TEST_MSG))
	if assert.Nil(t, err, "Err Write: %v", err) {
		assert.Equal(t, len(TEST_MSG), n, "Err Write nbyte: %v != %v", len(TEST_MSG), n)
		<-c
	}
	ts.Shutdown()
}

// Make sure failed dialing returns an error (connection refused)
func TestFailedDial(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, sp.INNER_CONTAINER_IP, PORT)
	ep := sp.NewEndpoint(sp.Taddrs{addr}, sp.ROOTREALM)
	err := ts.MintAndSetEndpointToken(ep)
	assert.Nil(t, err, "Err Mint EP token: %v", err)
	npc := ts.GetNetProxyClnt()
	// Dial an address with no corresponding listener
	_, err = npc.Dial(ep)
	assert.NotNil(t, err, "Err Dial: %v", err)
	ts.Shutdown()
}

// Make sure Listening works
func TestListen(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, sp.INNER_CONTAINER_IP, PORT)
	npc := ts.GetNetProxyClnt()
	// Create a listener via netproxy
	_, _, err := npc.Listen(addr)
	assert.Nil(t, err, "Err Listen: %v", err)
	ts.Shutdown()
}

// Make sure Listening on a random address returns an error
func TestFailedListen(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr("123.456.789.000", sp.INNER_CONTAINER_IP, PORT)
	npc := ts.GetNetProxyClnt()
	// Create a listener via netproxy
	_, _, err := npc.Listen(addr)
	assert.NotNil(t, err, "Err Listen: %v", err)
	ts.Shutdown()
}

// Make sure Close works
func TestClose(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, sp.INNER_CONTAINER_IP, PORT)
	npc := ts.GetNetProxyClnt()
	// Create a listener via netproxy
	ep, l, err := npc.Listen(addr)
	assert.Nil(t, err, "Err Listen: %v", err)
	err = l.Close()
	assert.Nil(t, err, "Err close: %v", err)
	// Dial the listener, to make sure it is unreachable
	_, err = npc.Dial(ep)
	assert.NotNil(t, err, "Err Dial: %v", err)
	ts.Shutdown()
}

// Make sure calling Close on an unknown listener returns an error
func TestFailedClose(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, sp.INNER_CONTAINER_IP, PORT)
	ep := sp.NewEndpoint(sp.Taddrs{addr}, sp.ROOTREALM)
	npc := ts.GetNetProxyClnt()
	l := netproxy.NewListener(npc, 1000, ep)
	err := l.Close()
	assert.NotNil(t, err, "Err close: %v", err)
	ts.Shutdown()
}

// Make sure Accept works
func TestAccept(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, sp.INNER_CONTAINER_IP, PORT)
	npc := ts.GetNetProxyClnt()
	c := make(chan bool)
	// Create a listener via netproxy
	ep, l, err := npc.Listen(addr)
	assert.Nil(t, err, "Err Listen: %v", err)
	go func(l net.Listener, c chan bool) {
		b := make([]byte, len(TEST_MSG))
		// Accept connections
		conn, err := l.Accept()
		assert.Nil(t, err, "Err accept: %v", err)
		n, err := conn.Read(b)
		assert.Nil(t, err, "Err read: %v", err)
		assert.Equal(t, len(b), n, "Err read nbyte: %v != %v", len(b), n)
		l.Close()
		c <- true
	}(l, c)
	// Dial the listener
	conn, err := npc.Dial(ep)
	assert.Nil(t, err, "Err Dial: %v", err)
	n, err := conn.Write([]byte(TEST_MSG))
	if assert.Nil(t, err, "Err Write: %v", err) {
		assert.Equal(t, len(TEST_MSG), n, "Err Write nbyte: %v != %v", len(TEST_MSG), n)
		<-c
	}
	ts.Shutdown()
}

// Make sure calling Accept on an unknown listener returns an error
func TestFailedAccept(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, sp.INNER_CONTAINER_IP, PORT)
	ep := sp.NewEndpoint(sp.Taddrs{addr}, sp.ROOTREALM)
	npc := ts.GetNetProxyClnt()
	l := netproxy.NewListener(npc, 1000, ep)
	_, err := l.Accept()
	assert.NotNil(t, err, "Err accept: %v", err)
	ts.Shutdown()
}
