package protclnt

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/lease"
	"ulambda/netclnt"
	np "ulambda/ninep"
)

// XXX duplicate
const (
	Msglen = 64 * 1024
)

type LeaseMap struct {
	sync.Mutex
	leases map[string]*lease.Lease
}

func MakeLeaseMap() *LeaseMap {
	lm := &LeaseMap{}
	lm.leases = make(map[string]*lease.Lease)
	return lm
}

func (lm *LeaseMap) Add(l *lease.Lease) error {
	lm.Lock()
	defer lm.Unlock()

	fn := np.Join(l.Fn)
	if _, ok := lm.leases[fn]; ok {
		return fmt.Errorf("%v already leased", fn)
	}
	lm.leases[fn] = l
	return nil
}

func (lm *LeaseMap) Present(path []string) bool {
	lm.Lock()
	defer lm.Unlock()

	fn := np.Join(path)
	_, ok := lm.leases[fn]
	return ok
}

func (lm *LeaseMap) Del(path []string) error {
	lm.Lock()
	defer lm.Unlock()

	fn := np.Join(path)
	if _, ok := lm.leases[fn]; !ok {
		return fmt.Errorf("%v no lease", fn)
	}
	delete(lm.leases, fn)
	return nil
}

type conn struct {
	nc *netclnt.NetClnt
	lm *LeaseMap
}

func makeConn(nc *netclnt.NetClnt) *conn {
	c := &conn{}
	c.lm = MakeLeaseMap()
	c.nc = nc
	return c
}

func (conn *conn) send(req np.Tmsg, session np.Tsession, seqno *np.Tseqno) (np.Tmsg, error) {
	reqfc := &np.Fcall{}
	reqfc.Type = req.Type()
	reqfc.Msg = req
	reqfc.Session = session
	reqfc.Seqno = seqno.Next()
	repfc, err := conn.nc.RPC(reqfc)
	if err != nil {
		return nil, err
	}
	return repfc.Msg, nil
}

type result struct {
	conn *conn
	err  error
}

func (conn *conn) aSend(ch chan result, dst []string, req np.Tmsg, s np.Tsession, seq *np.Tseqno) {
	log.Printf("aSend %v %v\n", dst, req)
	if reply, err := conn.send(req, s, seq); err != nil {
		log.Printf("aSend %v %v err %v\n", dst, req, err)
		ch <- result{conn, err}
	} else {
		if rmsg, ok := reply.(np.Rerror); ok {
			log.Printf("aSend err %v %v err %v\n", dst, req, rmsg.Ename)
			ch <- result{conn, errors.New(rmsg.Ename)}
		} else {
			ch <- result{conn, nil}
		}
	}
}

// XXX SessMgr?
type ConnMgr struct {
	mu      sync.Mutex
	session np.Tsession
	seqno   *np.Tseqno
	conns   map[string]*conn
}

func makeConnMgr(session np.Tsession, seqno *np.Tseqno) *ConnMgr {
	cm := &ConnMgr{}
	cm.conns = make(map[string]*conn)
	cm.session = session
	cm.seqno = seqno
	return cm
}

func (cm *ConnMgr) exit() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for addr, conn := range cm.conns {
		db.DLPrintf("9PCHAN", "exit close connection to %v\n", addr)
		conn.nc.Close()
		delete(cm.conns, addr)
	}
}

// XXX Make array
func (cm *ConnMgr) allocConn(addrs []string) (*conn, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Store as concatenation of addresses
	key := strings.Join(addrs, ",")

	if conn, ok := cm.conns[key]; ok {
		return conn, nil
	}
	nc, err := netclnt.MkNetClnt(addrs)
	if err != nil {
		return nil, err
	}
	cm.conns[key] = makeConn(nc)
	return cm.conns[key], err
}

func (cm *ConnMgr) lookupConn(addrs []string) (*conn, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, ok := cm.conns[strings.Join(addrs, ",")]
	return conn, ok
}

func (cm *ConnMgr) makeCall(dst []string, req np.Tmsg) (np.Tmsg, error) {
	conn, err := cm.allocConn(dst)
	if err != nil {
		return nil, err
	}
	return conn.send(req, cm.session, cm.seqno)
}

func (cm *ConnMgr) disconnect(dst []string) bool {
	conn, ok := cm.lookupConn(dst)
	if !ok {
		return false
	}
	conn.nc.Close()
	return true
}

// Multicasts a req on connections of cm. Caller specifies (1) ok
// func, which returns whether to send or not on a given conn, and (2)
// r to process the reply to a send.
func (cm *ConnMgr) mcastReq(req np.Tmsg, ok func(*conn) bool, r func(result) error) error {
	ch := make(chan result)
	cm.mu.Lock()

	log.Printf("%v: mcast %v %v %v\n", db.GetName(), len(cm.conns), req.Type(), req)

	n := 0
	for addr, conn := range cm.conns {
		if ok(conn) {
			n += 1
			go conn.aSend(ch, strings.Split(addr, ","), req, cm.session, cm.seqno)
		}
	}
	cm.mu.Unlock()

	var err error
	for i := 0; i < n; i++ {
		res := <-ch
		r(res)

		// Ignore EOF, since we cannot talk to that server
		// anymore.  We may try to reconnect and then we will
		// register again.
		if res.err != nil && res.err.Error() != "EOF" {
			err = res.err
		}
	}
	return err
}

func (cm *ConnMgr) registerLease(lease *lease.Lease) error {
	req := np.Tlease{lease.Fn, lease.Qid}
	err := cm.mcastReq(req,
		func(conn *conn) bool {
			return !conn.lm.Present(lease.Fn)
		},
		func(res result) error {
			if res.err == nil {
				log.Printf("add %p %v\n", res.conn, lease)
				return res.conn.lm.Add(lease)
			}
			return nil
		})
	return err
}

func (cm *ConnMgr) deregisterLease(path []string) error {
	req := np.Tunlease{path}
	err := cm.mcastReq(req,
		func(conn *conn) bool {
			return conn.lm.Present(path)
		},
		func(res result) error {
			if res.err == nil {
				return res.conn.lm.Del(path)
			}
			return nil
		})
	return err
}
