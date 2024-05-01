package netproxy_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	//	"sigmaos/fslib"
	//	"sigmaos/namesrv"
	//	"sigmaos/netproxy"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	IP       sp.Tip   = sp.LOCALHOST
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

func TestDial(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, sp.INNER_CONTAINER_IP, PORT)
	c := make(chan bool)
	l, err := net.Listen("tcp", addr.IPPort())
	assert.Nil(t, err, "Err Listen: %v", err)
	go func(l net.Listener, c chan bool) {
		b := make([]byte, len(TEST_MSG))
		conn, err := l.Accept()
		assert.Nil(t, err, "Err accept: %v", err)
		n, err := conn.Read(b)
		assert.Nil(t, err, "Err read: %v", err)
		assert.Equal(t, len(b), n, "Err read nbyte: %v != %v", len(b), n)
		c <- true
	}(l, c)
	ep := sp.NewEndpoint(sp.Taddrs{addr}, sp.ROOTREALM)
	err = ts.MintAndSetEndpointToken(ep)
	assert.Nil(t, err, "Err Mint EP token: %v", err)
	npc := ts.GetNetProxyClnt()
	conn, err := npc.Dial(ep)
	assert.Nil(t, err, "Err Dial: %v", err)
	n, err := conn.Write([]byte(TEST_MSG))
	if assert.Nil(t, err, "Err Write: %v", err) {
		assert.Equal(t, len(TEST_MSG), n, "Err Write nbyte: %v != %v", len(TEST_MSG), n)
		<-c
	}
	ts.Shutdown()
}
