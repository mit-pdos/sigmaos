package dialproxy_test

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
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
	addr := sp.NewTaddr(IP, PORT)
	ep := sp.NewEndpoint(sp.EXTERNAL_EP, sp.Taddrs{addr})
	npc := ts.GetDialProxyClnt()
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

func TestManyDial(t *testing.T) {
	const (
		NCLNT = 400
	)
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, PORT)
	c := make(chan bool)
	l, err := net.Listen("tcp", addr.IPPort())
	assert.Nil(t, err, "Err Listen: %v", err)
	go func(l net.Listener) {
		for true {
			conn, err := l.Accept()
			if err != nil {
				break
			}
			go func(conn net.Conn) {
				b := make([]byte, len(TEST_MSG))
				n, err := conn.Read(b)
				assert.Nil(t, err, "Err read: %v", err)
				assert.Equal(t, len(b), n, "Err read nbyte: %v != %v", len(b), n)
				c <- true
			}(conn)
		}
	}(l)
	for i := 0; i < NCLNT; i++ {
		go func(i int) {
			npc := ts.GetDialProxyClnt()
			ep := sp.NewEndpoint(sp.EXTERNAL_EP, sp.Taddrs{addr})
			conn, err := npc.Dial(ep)
			assert.Nil(t, err, "Err Dial: %v", err)
			n, err := conn.Write([]byte(TEST_MSG))
			if assert.Nil(t, err, "Err Write: %v", err) {
				assert.Equal(t, len(TEST_MSG), n, "Err Write nbyte: %v != %v", len(TEST_MSG), n)
			}
			conn.Close()
		}(i)
	}
	for i := 0; i < NCLNT; i++ {
		<-c
	}
	l.Close()
	ts.Shutdown()
}

// Make sure failed dialing returns an error (connection refused)
func TestFailedDial(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, PORT)
	ep := sp.NewEndpoint(sp.INTERNAL_EP, sp.Taddrs{addr})
	npc := ts.GetDialProxyClnt()
	// Dial an address with no corresponding listener
	_, err := npc.Dial(ep)
	assert.NotNil(t, err, "Err Dial: %v", err)
	ts.Shutdown()
}

// Make sure Listening works
func TestListen(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, PORT)
	npc := ts.GetDialProxyClnt()
	// Create a listener via dialproxy
	_, _, err := npc.Listen(sp.INTERNAL_EP, addr)
	assert.Nil(t, err, "Err Listen: %v", err)
	ts.Shutdown()
}

// Make sure Listening on a random address returns an error
func TestFailedListen(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr("123.456.789.000", PORT)
	npc := ts.GetDialProxyClnt()
	// Create a listener via dialproxy
	_, _, err := npc.Listen(sp.INTERNAL_EP, addr)
	assert.NotNil(t, err, "Err Listen: %v", err)
	ts.Shutdown()
}

// Make sure Close works
func TestClose(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, PORT)
	npc := ts.GetDialProxyClnt()
	// Create a listener via dialproxy
	ep, l, err := npc.Listen(sp.INTERNAL_EP, addr)
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
	addr := sp.NewTaddr(IP, PORT)
	ep := sp.NewEndpoint(sp.INTERNAL_EP, sp.Taddrs{addr})
	npc := ts.GetDialProxyClnt()
	l := dialproxyclnt.NewListener(npc, 1000, ep)
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
	addr := sp.NewTaddr(IP, PORT)
	npc := ts.GetDialProxyClnt()
	c := make(chan bool)
	// Create a listener via dialproxy
	ep, l, err := npc.Listen(sp.INTERNAL_EP, addr)
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
	addr := sp.NewTaddr(IP, PORT)
	ep := sp.NewEndpoint(sp.INTERNAL_EP, sp.Taddrs{addr})
	npc := ts.GetDialProxyClnt()
	l := dialproxyclnt.NewListener(npc, 1000, ep)
	_, err := l.Accept()
	assert.NotNil(t, err, "Err accept: %v", err)
	ts.Shutdown()
}

func TestNamedEndpoint(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	npc := ts.GetDialProxyClnt()
	ep, err := npc.GetNamedEndpoint(sp.ROOTREALM)
	assert.Nil(t, err, "GetNamedEndpoint: %v", err)
	db.DPrintf(db.TEST, "endpoint %v\n", ep)
	ts.Shutdown()
}

// Make sure client receives EOF when server closes connection
func TestServerClose(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, PORT)
	npc := ts.GetDialProxyClnt()
	// Create a listener via dialproxy
	ep, l, err := npc.Listen(sp.INTERNAL_EP, addr)
	assert.Nil(t, err, "Err Listen: %v", err)
	go func(l net.Listener) {
		b := make([]byte, len(TEST_MSG))
		// Accept connections
		conn, err := l.Accept()
		assert.Nil(t, err, "Err accept: %v", err)
		n, err := conn.Read(b)
		assert.Nil(t, err, "Err read: %v", err)
		assert.Equal(t, len(b), n, "Err read nbyte: %v != %v", len(b), n)
		conn.Close()
		db.DPrintf(db.TEST, "Server close %v", addr)
		l.Close()
	}(l)
	// Dial the listener
	conn, err := npc.Dial(ep)
	assert.Nil(t, err, "Err Dial: %v", err)
	n, err := conn.Write([]byte(TEST_MSG))
	if assert.Nil(t, err, "Err Write: %v", err) {
		assert.Equal(t, len(TEST_MSG), n, "Err Write nbyte: %v != %v", len(TEST_MSG), n)
		s := time.Now()
		b := make([]byte, len(TEST_MSG))
		_, err := conn.Read(b)
		assert.NotNil(t, err)
		assert.Equal(t, io.EOF, err)
		assert.True(t, time.Since(s).Seconds() < 2)
	}
	ts.Shutdown()
}

// Make sure server sees client close
func TestClientClose(t *testing.T) {
	ts, err1 := test.NewTstate(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	addr := sp.NewTaddr(IP, PORT)
	npc := ts.GetDialProxyClnt()
	// Create a listener via dialproxy
	ep, l, err := npc.Listen(sp.INTERNAL_EP, addr)
	assert.Nil(t, err, "Err Listen: %v", err)
	c := make(chan bool)
	go func(l net.Listener, ch chan bool) {
		b := make([]byte, len(TEST_MSG))
		// Accept connections
		conn, err := l.Accept()
		assert.Nil(t, err, "Err accept: %v", err)
		n, err := conn.Read(b)
		assert.Nil(t, err, "Err read: %v", err)
		assert.Equal(t, len(b), n, "Err read nbyte: %v != %v", len(b), n)
		// Wait for Read() to receive EOF
		s := time.Now()
		_, err = conn.Read(b)
		assert.NotNil(t, err)
		assert.True(t, time.Since(s).Seconds() < 2)
		ch <- true
		conn.Close()
		l.Close()
	}(l, c)
	// Dial the listener
	conn, err := npc.Dial(ep)
	assert.Nil(t, err, "Err Dial: %v", err)
	n, err := conn.Write([]byte(TEST_MSG))
	if assert.Nil(t, err, "Err Write: %v", err) {
		assert.Equal(t, len(TEST_MSG), n, "Err Write nbyte: %v != %v", len(TEST_MSG), n)
	}
	conn.Close()
	<-c
	ts.Shutdown()
}
