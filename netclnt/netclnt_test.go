package netclnt_test

import (
	"bufio"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/netclnt"
	"sigmaos/netsrv"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/sigmaprotsrv"
	"sigmaos/test"
)

const (
	BUFSZ = 64 * sp.KBYTE
	TOTAL = 1000 * sp.MBYTE
)

type call struct {
	buf []byte
}

func (c *call) Tag() sessp.Ttag {
	return 0
}

func ReadCall(rdr io.Reader) (demux.CallI, *serr.Err) {
	f, err := frame.ReadFrame(rdr)
	return &call{buf: f}, err
}

func WriteCall(wrt *bufio.Writer, c demux.CallI) *serr.Err {
	call := c.(*call)
	frame.WriteFrame(wrt, call.buf)
	return nil
}

type netSrv struct {
}

func (ns *netSrv) ServeRequest(c sigmaprotsrv.Conn, req demux.CallI) (demux.CallI, *serr.Err) {
	rep := &call{buf: make([]byte, 0)}
	return rep, nil
}

func (ns *netSrv) ReportError(c sigmaprotsrv.Conn, err error) {
}

type TstateNet struct {
	*test.TstateMin
	srv  *netsrv.NetServer
	clnt *netclnt.NetClnt
}

func (ts *TstateNet) ReportError(err error) {
}

func newTstateNet(t *testing.T) *TstateNet {
	ts := &TstateNet{}
	ts.TstateMin = test.NewTstateMin(t)

	ns := &netSrv{}
	ts.srv = netsrv.NewNetServer(ts.Pcfg, ns, ts.Addr, ReadCall, WriteCall)

	db.DPrintf(db.TEST, "srv %v\n", ts.srv.MyAddr())

	nc, err := netclnt.NewNetClnt(sp.ROOTREALM.String(), sp.Taddrs{ts.srv.MyAddr()}, ReadCall, WriteCall, ts)
	assert.Nil(t, err)
	ts.clnt = nc
	return ts
}

func TestNetClntPerf(t *testing.T) {
	ts := newTstateNet(t)
	c := &call{buf: test.NewBuf(BUFSZ)}

	t0 := time.Now()
	for i := 0; i < TOTAL/BUFSZ; i++ {
		_, err := ts.clnt.SendReceive(c)
		assert.Nil(t, err)
	}
	tot := uint64(TOTAL)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms (%v us per iter) tput %v\n", humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/BUFSZ), test.TputStr(TOTAL, ms))

	ts.srv.CloseListener()
}

func TestSocketPerf(t *testing.T) {
	const (
		SOCKPATH = "/tmp/test-perf-socket"
	)

	err := os.Remove(SOCKPATH)
	assert.True(t, err == nil || os.IsNotExist(err), "Err remove sock: %v", err)

	socket, err := net.Listen("unix", SOCKPATH)
	assert.Nil(t, err)
	err = os.Chmod(SOCKPATH, 0777)
	assert.Nil(t, err)

	buf := test.NewBuf(BUFSZ)
	ch := make(chan bool)
	// Serve requests in another thread
	go func() {
		conn, err := socket.Accept()
		assert.Nil(t, err)
		rdr := bufio.NewReaderSize(conn, BUFSZ)
		tot := 0
		for {
			rb := make([]byte, BUFSZ)
			n, err := io.ReadFull(rdr, rb)
			if err == io.EOF {
				db.DPrintf(db.TEST, "tot %d\n", tot)
				break
			}
			tot += n
			if n != len(rb) || err != nil {
				db.DFatalf("Err read: len %v err %v", n, err)
			}
			conn.Write(rb[0:1])
		}
		ch <- true

	}()

	conn, err := net.Dial("unix", SOCKPATH)
	assert.Nil(t, err)

	sz := sp.Tlength(TOTAL)

	t0 := time.Now()
	for i := 0; i < TOTAL/BUFSZ; i++ {
		n, err := conn.Write(buf)
		assert.Nil(t, err)
		assert.Equal(t, BUFSZ, n)
		rb := make([]byte, 1)
		_, err = conn.Read(rb)
		assert.Nil(t, err)
	}

	conn.Close()

	<-ch

	tot := uint64(sz)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms (%v us per iter) tput %v\n", humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/BUFSZ), test.TputStr(sz, ms))

	err = os.Remove(SOCKPATH)
	assert.True(t, err == nil || os.IsNotExist(err), "Err remove sock: %v", err)

	socket.Close()
}
