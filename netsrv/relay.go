package netsrv

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

const (
	NO_SEQNO       = 0
	DUMMY_RESPONSE = "DUMMY_RESPONSE"
)

type RelayOp struct {
	request      *np.Fcall
	requestFrame []byte
	reply        *np.Fcall
	replyFrame   []byte
	r            *RelayConn
	replies      chan *RelayOp
}

// A channel between clients & replicas, or replicas & replicas
type RelayConn struct {
	srv     *NetServer
	c       *SrvConn
	ops     chan *RelayOp
	replies chan *RelayOp
}

func MakeRelayConn(srv *NetServer, conn net.Conn) *RelayConn {
	protsrv := srv.fssrv.Connect()
	c := &SrvConn{sync.Mutex{},
		srv.fssrv,
		conn,
		false,
		protsrv,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		false,
		make(map[np.Tsession]bool),
	}
	r := &RelayConn{srv, c, srv.replState.ops, make(chan *RelayOp)}
	go r.writer()
	go r.reader()
	if atomic.CompareAndSwapUint32(&setupRelay, 0, 1) {
		r.setupRelay()
	}
	return r
}

func (r *RelayConn) reader() {
	db.DLPrintf("RSRV", "Conn from %v\n", r.c.Src())
	for {
		frame, err := npcodec.ReadFrame(r.c.br)
		if err != nil {
			db.DLPrintf("RSRV", "%v Peer %v closed/erred %v\n", r.c.Dst(), r.c.Src(), err)
			if err == io.EOF {
				r.c.close()
			}
			return
		}
		db.DLPrintf("RSRV", "%v relay reader read frame from %v\n", r.c.Dst(), r.c.Src())
		fcall := &np.Fcall{}
		if err := npcodec.Unmarshal(frame, fcall); err != nil {
			log.Printf("Server %v: relayWriter unmarshal error: %v", r.c.Dst(), err)
			// TODO: enqueue op with empty reply
		} else {
			op := &RelayOp{fcall, frame, nil, []byte{}, r, r.replies}
			r.ops <- op
		}
	}
}

func (r *RelayConn) serve(fc *np.Fcall) *np.Fcall {
	t := fc.Tag
	// XXX Avoid doing this every time
	r.c.fssrv.SessionTable().RegisterSession(fc.Session)
	reply, rerror := r.c.dispatch(fc.Session, fc.Msg)
	if rerror != nil {
		reply = *rerror
	}
	fcall := &np.Fcall{}
	fcall.Type = reply.Type()
	fcall.Msg = reply
	fcall.Tag = t
	return fcall
}

func (r *RelayConn) writer() {
	for {
		op, ok := <-r.replies
		if !ok {
			return
		}
		db.DLPrintf("RSRV", "%v -> %v relay writer reply: %v", r.c.Dst(), r.c.Src(), op.reply)
		err := npcodec.WriteRawBuffer(r.c.bw, op.replyFrame)
		if err != nil {
			db.DLPrintf("RSRV", "%v -> %v Writer: WriteFrame error: %v", r.c.Src(), r.c.Dst(), err)
			continue
		}
		err = r.c.bw.Flush()
		if err != nil {
			db.DLPrintf("RSRV", "%v -> %v Writer: Flush error: %v", r.c.Src(), r.c.Dst(), err)
			continue
		}
	}
}

func (r *RelayConn) setupRelay() {
	// Run a worker to process messages
	go r.relayReader()
	// Run a worker to dispatch responses
	go r.relayWriter()
}

func (r *RelayConn) cacheReply(request *np.Fcall, reply *np.Fcall) {
	var replyFrame []byte
	var replyBuffer bytes.Buffer
	bw := bufio.NewWriter(&replyBuffer)
	err := npcodec.MarshalFcallToWriter(reply, bw)
	if err != nil {
		log.Printf("Error marshaling reply: %v", err)
	}
	bw.Flush()
	replyFrame = replyBuffer.Bytes()
	r.srv.replState.replyCache.Put(request, reply, replyFrame)
}

func (r *RelayConn) relayReader() {
	config := r.srv.replConfig
	state := r.srv.replState
	for {
		op, ok := <-state.ops
		if !ok {
			return
		}
		// If this was a duplicate reply from the cache
		if reply, ok := r.srv.replState.replyCache.Get(op.request); ok {
			op.reply = reply.fcall
			op.replyFrame = reply.frame
			db.DLPrintf("RSRV", "%v Dup relay request %v", config.RelayAddr, op.request)
			// We have already seen this request. 2 Options:
			// 1. If it's in-flight (we haven't seen an ack from the tail yet), we
			//    need to register the op in order to have our ack thread send a
			//    reply.
			// 2. If it isn't in flight (we have already seen an ack from the tail
			//    for this request), we need to respond immediately with a response
			//    from the reply cache.
			// Note that we do *not* need to resend the request, as our reliable
			// send mechanism will take care of this (and on configuration switch,
			// the request will be resent automatically).
			//
			// AddIfDuplicate is a CAS-like function which atomically checks if the op
			// is in-flight, and if so, adds this duplicate to the set and returns
			// true. Otherwise, it returns false. This atomicity is needed to make
			// sure we never drop acks which should be relayed upstream.
			// Tail acks taken care of separately
			if !r.srv.isTail() {
				// XXX Could there be a race here?
				if state.inFlight.AddIfDuplicate(op) {
					db.DLPrintf("RSRV", "%v Added dup in-flight request: %v", config.RelayAddr, op.request)
				} else {
					db.DLPrintf("RSRV", "%v Dup request not in-flight, replying immediately. req: %v rep: %v", config.RelayAddr, op.request, op.reply)
					op.replies <- op
				}
			}
		} else {
			// We make the simplifying assumption that all replies are in the
			// replyCache right now. This will need to be revised when we start
			// evicting from the cache. Specifically, we need to handle edge cases
			// where messages with older sequence numbers (since seqnos are now
			// assigned by the client) may be delayed for a long time.
			db.DLPrintf("RSRV", "%v Reader relay request %v", config.RelayAddr, op.request)
			// Serve the op first.
			op.reply = op.r.serve(op.request)
			op.reply.Session = op.request.Session
			op.reply.Seqno = op.request.Seqno
			db.DLPrintf("RSRV", "%v Reader relay reply %v", config.RelayAddr, op.reply)
			// Cache the reply
			r.cacheReply(op.request, op.reply)
			cachedReply, _ := r.srv.replState.replyCache.Get(op.request)
			op.replyFrame = cachedReply.frame
			// Optionally log the fcall & its reply type.
			r.srv.logOp(op.request, op.reply)
			// Reliably send to the next link in the chain (even if that link
			// changes)
			if !r.srv.isTail() {
				// Enqueue the message to mark it as in-flight.
				state.inFlight.Add(op)
				r.relayOp(op)
			}
		}
		// If we're the tail, we always ack immediately
		if r.srv.isTail() {
			db.DLPrintf("RSRV", "%v Tail acking %v", config.RelayAddr, op.request)
			op.replies <- op
		}
	}
}

// Relay acks upstream.
func (r *RelayConn) relayWriter() {
	config := r.srv.replConfig
	state := r.srv.replState
	for {
		// XXX Don't spin
		if r.srv.isTail() {
			continue
		}
		state.mu.Lock()
		ch := state.NextChan
		nextAddr := config.NextAddr
		state.mu.Unlock()
		// Channels start as nil during initialization.
		if ch == nil {
			continue
		}
		db.DLPrintf("RSRV", "%v Recv from downstream %v", config.RelayAddr, nextAddr)
		// Get an ack from the downstream servers
		frame, err := ch.Recv()
		db.DLPrintf("RSRV", "%v Recv'd downstream from %v", config.RelayAddr, nextAddr)
		// Move on if the connection closed
		if peerCrashed(err) {
			db.DLPrintf("RSRV", "%v error relayWriter Recv: %v", config.RelayAddr, err)
			continue
		}
		if err != nil {
			log.Printf("%v error receiving ack: %v\n", config.RelayAddr, err)
			continue
		}

		ack := &np.Fcall{}
		if err := npcodec.Unmarshal(frame, ack); err != nil {
			log.Printf("Error unmarshalling in relayWriter: %v", err)
			log.Printf("Frame: %v, len: %v", frame, len(frame))
		} else {
			db.DLPrintf("RSRV", "%v Got ack: %v", config.RelayAddr, ack)
			// Dequeue all acks up until this one (they may come out of order, which is
			// OK.
			ops := state.inFlight.Remove(ack)
			db.DLPrintf("RSRV", "%v Removed ack'd ops: %v", config.RelayAddr, ops)
			// Ack upstream
			for _, op := range ops {
				op.replies <- op
			}
		}
	}
}

func (r *RelayConn) relayOp(op *RelayOp) {
	config := r.srv.replConfig
	state := r.srv.replState

	// Get the next channel & address of the last person we sent to...
	state.mu.Lock()
	ch := state.NextChan
	nextAddr := config.NextAddr
	lastSendAddr := config.LastSendAddr
	state.mu.Unlock()

	// If the next server has changed (detected by config swap, or message send
	// failure), resend all in-flight requests. Should already include this
	// message.
	db.DLPrintf("RSRV", "%v -> %v Sending initial relayOp: %v", config.RelayAddr, nextAddr, op)
	if lastSendAddr != nextAddr || !relayOnce(r.srv, ch, op) {
		resendInflightRelayOps(r.srv)
	}
}

func resendInflightRelayOps(srv *NetServer) {
	config := srv.replConfig
	state := srv.replState

	// Get the connection to the next server, and reflect that we've sent to it.
	state.mu.Lock()
	ch := state.NextChan
	config.LastSendAddr = config.NextAddr
	nextAddr := config.NextAddr
	state.mu.Unlock()

	toSend := state.inFlight.GetOps()
	db.DLPrintf("RSRV", "%v -> %v Resending inflight messages: %v", config.RelayAddr, nextAddr, toSend)
	// Retry. On failure, resend all messages which haven't been ack'd, plus msg.
	for len(toSend) != 0 {
		// We shouldn't send anything if we're the tail
		if srv.isTail() {
			db.DLPrintf("RSRV", "%v -> %v Was tail, cancelling resend", config.RelayAddr, nextAddr)
			return
		}
		// Try to send a message.
		if ok := relayOnce(srv, ch, toSend[0]); ok {
			// If successful, move on to the next one
			toSend = toSend[1:]
		} else {
			// Else, retry sending the whole queue again
			state.mu.Lock()
			ch = state.NextChan
			config.LastSendAddr = config.NextAddr
			state.mu.Unlock()
			toSend = state.inFlight.GetOps()
			db.DLPrintf("RSRV", "%v -> %v Resending inflight messages (retry): %v", config.RelayAddr, nextAddr, toSend)
		}
	}
	db.DLPrintf("RSRV", "%v Done Resending inflight messages to %v", config.RelayAddr, nextAddr)
}

func sendAllAcks(srv *NetServer) {
	config := srv.replConfig
	state := srv.replState

	ops := state.inFlight.RemoveAll()
	db.DLPrintf("RSRV", "%v Sent all acks: %v", config.RelayAddr, ops)
	// Ack upstream
	go func() {
		for _, op := range ops {
			op.replies <- op
		}
	}()
}

// Try and send a message to the next server in the chain, and receive a
// response.
func relayOnce(srv *NetServer, ch *RelayNetConn, op *RelayOp) bool {
	// Only call down the chain if we aren't at the tail.
	// XXX Get rid of this if
	if !srv.isTail() {
		var err error
		// Just pass wrapped op along...
		err = ch.Send(op.requestFrame)
		// If the next server has crashed, note failure...
		if peerCrashed(err) {
			db.DLPrintf("RSRV", "%v sending error: %v", srv.replConfig.RelayAddr, err)
			return false
		}
		if err != nil {
			log.Fatalf("Srv error sending: %v\n", err)
		}
	} else {
		db.DLPrintf("%v Tail trying to relay", srv.replConfig.RelayAddr)
		return false
	}
	// If we made it this far, the send was successful
	return true
}

// Log an op & the type of the reply. Logging the exact reply is not useful,
// since contents may vary between replicas (e.g. time)
func (srv *NetServer) logOp(request *np.Fcall, reply *np.Fcall) {
	config := srv.replConfig
	state := srv.replState

	if config.LogOps {
		fpath := "name/" + config.RelayAddr + "-log.txt"
		b, err := state.ReadFile(fpath)
		if err != nil {
			log.Printf("Error reading log file in logOp: %v", err)
		}
		frame, err := npcodec.Marshal(request)
		if err != nil {
			log.Printf("Error marshalling request in logOp: %v", err)
		}
		b = append(b, frame...)
		b = append(b, []byte(request.Type.String())...)
		b = append(b, []byte(reply.Type.String())...)
		err = state.WriteFile(fpath, b)
		if err != nil {
			log.Printf("Error writing log file in logOp: %v", err)
		}
	}
}

func (op *RelayOp) String() string {
	return fmt.Sprintf("{ request:%v reply:%v }", op.request, op.reply)
}

func peerCrashed(err error) bool {
	return err != nil &&
		(err.Error() == "EOF" ||
			strings.Contains(err.Error(), "use of closed network connection") ||
			strings.Contains(err.Error(), "connection reset by peer"))
}
