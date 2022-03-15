package sessionclnt

import (
	"log"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/netclnt"
	np "ulambda/ninep"
)

// XXX duplicate
const (
	Msglen = 64 * 1024
)

/*
 * TODO
 * - Send heartbeats.
 * - Lift re-sending code into this package.
 */
type SessClnt struct {
	mu      sync.Mutex
	session np.Tsession
	seqno   *np.Tseqno
	conns   map[string]*conn // XXX Is a SessClnt ever used to talk to multiple servers?
}

func MakeSessClnt(session np.Tsession, seqno *np.Tseqno) *SessClnt {
	sc := &SessClnt{}
	sc.conns = make(map[string]*conn)
	sc.session = session
	sc.seqno = seqno
	return sc
}

func (sc *SessClnt) Exit() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	for addr, conn := range sc.conns {
		db.DLPrintf("SESSCLNT", "exit close connection to %v\n", addr)
		conn.close()
		delete(sc.conns, addr)
	}
}

// Return an existing conn if there is one, else allocate a new one. Caller
// holds lock.
func (sc *SessClnt) allocConn(addrs []string) (*conn, *np.Err) {
	// Store as concatenation of addresses
	key := connKey(addrs)
	if conn, ok := sc.conns[key]; ok {
		return conn, nil
	}
	conn, err := makeConn(addrs)
	if err != nil {
		return nil, err
	}
	sc.conns[key] = conn
	return conn, nil
}

func (sc *SessClnt) RPC(addrs []string, req np.Tmsg) (np.Tmsg, *np.Err) {
	// Get or establish connection
	conn, err := sc.allocConn(addrs)
	if err != nil {
		log.Printf("Error %v Unable to connect to %v", err, addrs)
		db.DLPrintf("SESSCLNT", "Unable to send request to %v\n", err, addrs)
		return nil, err
	}
	// Take the lock to ensure requests are sent in order of seqno.
	sc.mu.Lock()
	rpc := netclnt.MakeRpc(np.MakeFcall(req, sc.session, sc.seqno))
	// Reliably send the RPC to a replica. If the replica becomes unavailable,
	// this request will be resent.
	if err := conn.send(rpc); err != nil {
		log.Printf("Error %v Unable to send request to %v", err, addrs)
		db.DLPrintf("SESSCLNT", "Unable to send request to %v\n", err, addrs)
		return nil, err
	}
	sc.mu.Unlock()

	// Reliably receive a response from one of the replicas.
	reply, err := conn.recv(rpc)
	if err != nil {
		log.Printf("Error %v Unable to recv response from %v", err, addrs)
		db.DLPrintf("SESSCLNT", "Unable to recv response from %v\n", err, addrs)
		return nil, err
	}
	return reply, nil
}

func (sc *SessClnt) Disconnect(addrs []string) *np.Err {
	key := connKey(addrs)
	sc.mu.Lock()
	conn, ok := sc.conns[key]
	sc.mu.Unlock()
	if !ok {
		return np.MkErr(np.TErrUnreachable, connKey(addrs))
	}
	conn.close()
	delete(sc.conns, key)
	return nil
}

func connKey(addrs []string) string {
	return strings.Join(addrs, ",")
}
