// The netclnt package establishes a TCP connection and use [demux] to
// sends and receive request/responses to a single server.
package netclnt

import (
	"bufio"
	"net"
	"sync"

	// "time"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/netsigma"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type NetClnt struct {
	*demux.DemuxClnt
	mu     sync.Mutex
	conn   net.Conn
	addr   *sp.Taddr
	closed bool
	realm  sp.Trealm
}

func NewNetClnt(clntnet string, addrs sp.Taddrs, rf demux.ReadCallF, wf demux.WriteCallF, clnti demux.DemuxClntI) (*NetClnt, *serr.Err) {
	db.DPrintf(db.NETCLNT, "NewNetClnt to %v\n", addrs)
	nc := &NetClnt{}
	bw, br, err := nc.connect(clntnet, addrs)
	if err != nil {
		db.DPrintf(db.NETCLNT_ERR, "NewNetClnt connect %v err %v\n", addrs, err)
		return nil, err
	}
	nc.DemuxClnt = demux.NewDemuxClnt(bw, br, rf, wf, clnti)
	return nc, nil
}

func (nc *NetClnt) Dst() string {
	return nc.conn.RemoteAddr().String()
}

func (nc *NetClnt) Src() string {
	return nc.conn.LocalAddr().String()
}

// Close connection. This also causes dmxclnt to be closed
func (nc *NetClnt) Close() error {
	return nc.conn.Close()
}

func (nc *NetClnt) connect(clntnet string, addrs sp.Taddrs) (*bufio.Writer, *bufio.Reader, *serr.Err) {
	addrs = netsigma.Rearrange(clntnet, addrs)
	db.DPrintf(db.PORT, "NetClnt %v connect to any of %v, starting w. %v\n", clntnet, addrs, addrs[0])
	for _, addr := range addrs {
		c, err := net.DialTimeout("tcp", addr.IPPort(), sp.Conf.Session.TIMEOUT/10)
		db.DPrintf(db.PORT, "Dial %v addr.Addr %v\n", addr.IPPort(), err)
		if err != nil {
			continue
		}
		nc.conn = c
		nc.addr = addr
		br := bufio.NewReaderSize(c, sp.Conf.Conn.MSG_LEN)
		bw := bufio.NewWriterSize(c, sp.Conf.Conn.MSG_LEN)
		db.DPrintf(db.PORT, "NetClnt connected %v -> %v\n", c.LocalAddr(), nc.addr)
		return bw, br, nil
	}
	db.DPrintf(db.NETCLNT_ERR, "NetClnt unable to connect to any of %v\n", addrs)
	return nil, nil, serr.NewErr(serr.TErrUnreachable, "no connection")
}
