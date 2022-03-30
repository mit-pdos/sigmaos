package replchain

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/protsrv"
	"ulambda/repl"
)

const (
	Msglen = 64 * 1024
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
	srv     *ChainReplServer
	fssrv   protsrv.FsServer
	np      protsrv.Protsrv
	conn    net.Conn
	br      *bufio.Reader
	bw      *bufio.Writer
	ops     chan *RelayOp
	replies chan *RelayOp
}

func (rs *ChainReplServer) MakeConn(psrv protsrv.FsServer, conn net.Conn) repl.Conn {
	r := &RelayConn{
		rs,
		psrv, nil, conn, // FIXME psrv?
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		rs.ops, make(chan *RelayOp)}
	go r.writer()
	go r.reader()
	return r
}

func (r *RelayConn) reader() {
	db.DPrintf("RSRV", "Conn from %v\n", r.Src())
	for {
		frame, err := npcodec.ReadFrame(r.br)
		if err != nil {
			db.DPrintf("RSRV", "%v Peer %v closed/erred %v\n", r.Dst(), r.Src(), err)
			if err == io.EOF {
				r.close()
			}
			return
		}
		db.DPrintf("RSRV", "%v relay reader read frame from %v\n", r.Dst(), r.Src())
		fcall := &np.Fcall{}
		if fcall, err := npcodec.UnmarshalFcall(frame); err != nil {
			log.Printf("Server %v: relayWriter unmarshal error: %v", r.Dst(), err)
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

	// XXX fix me
	var reply np.Tmsg
	//reply, rerror := r.fssrv.Process(fc.Session, fc.Msg)
	//if rerror != nil {
	//	reply = *rerror
	//}
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
		db.DPrintf("RSRV", "%v -> %v relay writer reply: %v", r.Dst(), r.Src(), op.reply)
		err := npcodec.WriteRawBuffer(r.bw, op.replyFrame)
		if err != nil {
			db.DPrintf("RSRV", "%v -> %v Writer: WriteFrame error: %v", r.Src(), r.Dst(), err)
			continue
		}
		error := r.bw.Flush()
		if error != nil {
			db.DPrintf("RSRV", "%v -> %v Writer: Flush error: %v", r.Src(), r.Dst(), err)
			continue
		}
	}
}

func (rs *ChainReplServer) setupRelay() {
	// Run a worker to process messages
	go rs.relayReader()
	// Run a worker to dispatch responses
	go rs.relayWriter()
}

func (rs *ChainReplServer) cacheReply(request *np.Fcall, reply *np.Fcall) {
	var replyFrame []byte
	var replyBuffer bytes.Buffer
	bw := bufio.NewWriter(&replyBuffer)
	err := npcodec.MarshalFcallToWriter(reply, bw)
	if err != nil {
		log.Printf("Error marshaling reply: %v", err)
	}
	bw.Flush()
	replyFrame = replyBuffer.Bytes()
	rs.replyCache.Put(request, reply, replyFrame)
}

func (rs *ChainReplServer) relayReader() {
	config := rs.config
	for {
		op, ok := <-rs.ops
		if !ok {
			return
		}
		// If this was a duplicate reply from the cache
		if reply, ok := rs.replyCache.Get(op.request); ok {
			op.reply = reply.fcall
			op.replyFrame = reply.frame
			db.DPrintf("RSRV", "%v Dup relay request %v", config.RelayAddr, op.request)
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
			if !rs.isTail() {
				// XXX Could there be a race here?
				if rs.inFlight.AddIfDuplicate(op) {
					db.DPrintf("RSRV", "%v Added dup in-flight request: %v", config.RelayAddr, op.request)
				} else {
					db.DPrintf("RSRV", "%v Dup request not in-flight, replying immediately. req: %v rep: %v", config.RelayAddr, op.request, op.reply)
					op.replies <- op
				}
			}
		} else {
			// We make the simplifying assumption that all replies are in the
			// replyCache right now. This will need to be revised when we start
			// evicting from the cache. Specifically, we need to handle edge cases
			// where messages with older sequence numbers (since seqnos are now
			// assigned by the client) may be delayed for a long time.
			db.DPrintf("RSRV", "%v Reader relay request %v", config.RelayAddr, op.request)
			// Serve the op first.
			op.reply = op.r.serve(op.request)
			op.reply.Session = op.request.Session
			op.reply.Seqno = op.request.Seqno
			db.DPrintf("RSRV", "%v Reader relay reply %v", config.RelayAddr, op.reply)
			// Cache the reply
			rs.cacheReply(op.request, op.reply)
			cachedReply, _ := rs.replyCache.Get(op.request)
			op.replyFrame = cachedReply.frame
			// Optionally log the fcall & its reply type.
			rs.logOp(op.request, op.reply)
			// Reliably send to the next link in the chain (even if that link
			// changes)
			if !rs.isTail() {
				// Enqueue the message to mark it as in-flight.
				rs.inFlight.Add(op)
				rs.relayOp(op)
			}
		}
		// If we're the tail, we always ack immediately
		if rs.isTail() {
			db.DPrintf("RSRV", "%v Tail acking %v", config.RelayAddr, op.request)
			op.replies <- op
		}
	}
}

func (r *RelayConn) Src() string {
	return r.conn.RemoteAddr().String()
}

func (r *RelayConn) Dst() string {
	return r.conn.LocalAddr().String()
}

func (r *RelayConn) close() {
	db.DPrintf("RELAYCONN", "Close: %v", r.Src())
}

// Relay acks upstream.
func (rs *ChainReplServer) relayWriter() {
	config := rs.config
	for {
		// XXX Don't spin
		if rs.isTail() {
			continue
		}
		rs.mu.Lock()
		ch := rs.NextChan
		nextAddr := config.NextAddr
		rs.mu.Unlock()
		// Channels start as nil during initialization.
		if ch == nil {
			continue
		}
		db.DPrintf("RSRV", "%v Recv from downstream %v", config.RelayAddr, nextAddr)
		// Get an ack from the downstream servers
		frame, err := ch.Recv()
		db.DPrintf("RSRV", "%v Recv'd downstream from %v", config.RelayAddr, nextAddr)
		// Move on if the connection closed
		if peerCrashed(err) {
			db.DPrintf("RSRV", "%v error relayWriter Recv: %v", config.RelayAddr, err)
			continue
		}
		if err != nil {
			log.Printf("%v error receiving ack: %v\n", config.RelayAddr, err)
			continue
		}

		ack := &np.Fcall{}
		if ack, err := npcodec.UnmarshalFcall(frame); err != nil {
			log.Printf("Error unmarshalling in relayWriter: %v", err)
			log.Printf("Frame: %v, len: %v", frame, len(frame))
		} else {
			db.DPrintf("RSRV", "%v Got ack: %v", config.RelayAddr, ack)
			// Dequeue all acks up until this one (they may come out of order, which is
			// OK.
			ops := rs.inFlight.Remove(ack)
			db.DPrintf("RSRV", "%v Removed ack'd ops: %v", config.RelayAddr, ops)
			// Ack upstream
			for _, op := range ops {
				op.replies <- op
			}
		}
	}
}

func (rs *ChainReplServer) relayOp(op *RelayOp) {
	config := rs.config

	// Get the next channel & address of the last person we sent to...
	rs.mu.Lock()
	ch := rs.NextChan
	nextAddr := config.NextAddr
	lastSendAddr := config.LastSendAddr
	rs.mu.Unlock()

	// If the next server has changed (detected by config swap, or message send
	// failure), resend all in-flight requests. Should already include this
	// message.
	db.DPrintf("RSRV", "%v -> %v Sending initial relayOp: %v", config.RelayAddr, nextAddr, op)
	if lastSendAddr != nextAddr || !relayOnce(rs, ch, op) {
		resendInflightRelayOps(rs)
	}
}

func resendInflightRelayOps(rs *ChainReplServer) {
	config := rs.config

	// Get the connection to the next server, and reflect that we've sent to it.
	rs.mu.Lock()
	ch := rs.NextChan
	config.LastSendAddr = config.NextAddr
	nextAddr := config.NextAddr
	rs.mu.Unlock()

	toSend := rs.inFlight.GetOps()
	db.DPrintf("RSRV", "%v -> %v Resending inflight messages: %v", config.RelayAddr, nextAddr, toSend)
	// Retry. On failure, resend all messages which haven't been ack'd, plus msg.
	for len(toSend) != 0 {
		// We shouldn't send anything if we're the tail
		if rs.isTail() {
			db.DPrintf("RSRV", "%v -> %v Was tail, cancelling resend", config.RelayAddr, nextAddr)
			return
		}
		// Try to send a message.
		if ok := relayOnce(rs, ch, toSend[0]); ok {
			// If successful, move on to the next one
			toSend = toSend[1:]
		} else {
			// Else, retry sending the whole queue again
			rs.mu.Lock()
			ch = rs.NextChan
			config.LastSendAddr = config.NextAddr
			rs.mu.Unlock()
			toSend = rs.inFlight.GetOps()
			db.DPrintf("RSRV", "%v -> %v Resending inflight messages (retry): %v", config.RelayAddr, nextAddr, toSend)
		}
	}
	db.DPrintf("RSRV", "%v Done Resending inflight messages to %v", config.RelayAddr, nextAddr)
}

func sendAllAcks(rs *ChainReplServer) {
	config := rs.config

	ops := rs.inFlight.RemoveAll()
	db.DPrintf("RSRV", "%v Sent all acks: %v", config.RelayAddr, ops)
	// Ack upstream
	go func() {
		for _, op := range ops {
			op.replies <- op
		}
	}()
}

// Try and send a message to the next server in the chain, and receive a
// response.
func relayOnce(rs *ChainReplServer, ch *RelayNetConn, op *RelayOp) bool {
	// Only call down the chain if we aren't at the tail.
	// XXX Get rid of this if
	if !rs.isTail() {
		var err error
		// Just pass wrapped op along...
		err = ch.Send(op.requestFrame)
		// If the next server has crashed, note failure...
		if peerCrashed(err) {
			db.DPrintf("RSRV", "%v sending error: %v", rs.config.RelayAddr, err)
			return false
		}
		if err != nil {
			log.Fatalf("Srv error sending: %v\n", err)
		}
	} else {
		db.DPrintf("%v Tail trying to relay", rs.config.RelayAddr)
		return false
	}
	// If we made it this far, the send was successful
	return true
}

// Log an op & the type of the reply. Logging the exact reply is not useful,
// since contents may vary between replicas (e.g. time)
func (rs *ChainReplServer) logOp(request *np.Fcall, reply *np.Fcall) {
	config := rs.config

	if config.LogOps {
		fpath := "name/" + config.RelayAddr + "-log.txt"
		b, err := rs.ReadFile(fpath)
		if err != nil {
			log.Printf("Error reading log file in logOp: %v", err)
		}
		frame, err := npcodec.MarshalFcallByte(request)
		if err != nil {
			log.Printf("Error marshalling request in logOp: %v", err)
		}
		b = append(b, frame...)
		b = append(b, []byte(request.Type.String())...)
		b = append(b, []byte(reply.Type.String())...)
		err = rs.WriteFile(fpath, b)
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
