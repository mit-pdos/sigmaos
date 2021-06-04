package npsrv

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	//	"os"
	//	"os/user"
	//	"path"
	//	"runtime/pprof"
	"strings"
	"sync"
	//	"time"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/npobjsrv"
)

const (
	NO_SEQNO       = 0
	DUMMY_RESPONSE = "DUMMY_RESPONSE"
)

type FcallWrapper struct {
	Seqno uint64
	Fcall *np.Fcall
}

type SrvOp struct {
	wrapped bool
	seqno   uint64
	frame   []byte
	reply   *np.Fcall
	r       *RelayChannel
	// XXX get rid of this chan
	replies chan *SrvOp
}

// A channel between clients & replicas, or replicas & replicas
type RelayChannel struct {
	c       *Channel
	ops     chan *SrvOp
	replies chan *SrvOp
	wrapped bool
}

func MakeRelayChannel(npc NpConn, conn net.Conn, ops chan *SrvOp, wrapped bool, fids map[np.Tfid]*npobjsrv.Fid) *RelayChannel {
	npapi := npc.Connect(conn)
	n := npapi.(*npobjsrv.NpConn)
	// Keep a single Fid table around for all connections between this replica and
	// the previous one, even if that replica changes, since clients expect Fids
	// to remain valid all along the chain even in the presence of failures, and
	// they're normally per-connection (and therefore reset when a replica fails).
	n.SetFids(fids)
	c := &Channel{sync.Mutex{},
		npc,
		conn,
		false,
		n,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		false,
	}
	r := &RelayChannel{c, ops, make(chan *SrvOp), wrapped}
	go r.writer()
	go r.reader()
	return r
}

func (r *RelayChannel) reader() {
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
		op := &SrvOp{r.wrapped, NO_SEQNO, frame, nil, r, r.replies}
		r.ops <- op
	}
}

func (r *RelayChannel) serve(fc *np.Fcall) *np.Fcall {
	t := fc.Tag
	reply, rerror := r.c.dispatch(fc.Msg)
	if rerror != nil {
		reply = *rerror
	}
	fcall := &np.Fcall{}
	fcall.Type = reply.Type()
	fcall.Msg = reply
	fcall.Tag = t
	return fcall
}

func (r *RelayChannel) writer() {
	for {
		op, ok := <-r.replies
		if !ok {
			return
		}

		db.DLPrintf("RSRV", "%v marshalling op %v", r.c.Dst(), op)
		frame, err := marshalFcall(op.reply, op.wrapped, op.seqno)

		sendBuf := false
		var data []byte
		switch op.reply.Type {
		case np.TTwrite:
			msg := op.reply.Msg.(np.Twrite)
			data = msg.Data
			sendBuf = true
		case np.TTwritev:
			msg := op.reply.Msg.(np.Twritev)
			data = msg.Data
			sendBuf = true
		case np.TRread:
			msg := op.reply.Msg.(np.Rread)
			data = msg.Data
			sendBuf = true
		default:
		}

		if sendBuf {
			err = npcodec.WriteFrameAndBuf(r.c.bw, frame, data)
		} else {
			err = npcodec.WriteFrame(r.c.bw, frame)
		}

		//		err := npcodec.WriteFrame(r.c.bw, frame)
		if err != nil {
			db.DLPrintf("RSRV", "%v -> %v Writer: WriteFrame error: %v", r.c.Src(), r.c.Dst(), err)
			return
		}
		err = r.c.bw.Flush()
		if err != nil {
			db.DLPrintf("RSRV", "%v -> %v Writer: Flush error: %v", r.c.Src(), r.c.Dst(), err)
			return
		}
	}
}

func marshalFcall(fcall *np.Fcall, wrapped bool, seqno uint64) ([]byte, error) {
	var b []byte
	var err error
	if wrapped {
		buf := bytes.Buffer{}
		wrap := &FcallWrapper{seqno, fcall}
		e := gob.NewEncoder(&buf)
		err = e.Encode(wrap)
		b = buf.Bytes()
	} else {
		b, err = npcodec.Marshal(fcall)
	}
	return b, err
}

func unmarshalFcall(frame []byte, wrapped bool) (*FcallWrapper, error) {
	wrap := &FcallWrapper{}
	var err error
	if wrapped {
		buf := bytes.NewBuffer(frame)
		d := gob.NewDecoder(buf)
		err = d.Decode(wrap)
	} else {
		wrap.Seqno = NO_SEQNO
		wrap.Fcall = &np.Fcall{}
		err = npcodec.Unmarshal(frame, wrap.Fcall)
	}
	return wrap, err
}

func (srv *NpServer) setupRelay() {
	// Run a worker to process messages
	go srv.relayReader()
	// Run a worker to dispatch responses
	go srv.relayWriter()
}

func (srv *NpServer) relayReader() {

	//	// ===== Profiling code =====
	//	usr, err := user.Current()
	//	if err != nil {
	//		log.Fatalf("Error getting current user: %v", err)
	//	}
	//	f, err := os.Create(path.Join(usr.HomeDir, "replica-"+srv.replConfig.RelayAddr+".out"))
	//	if err != nil {
	//		log.Printf("Couldn't make profile file")
	//	}
	//	defer f.Close()
	//	if err := pprof.StartCPUProfile(f); err != nil {
	//		log.Fatalf("Couldn't start CPU profile: %v", err)
	//	}
	//	go func() {
	//		defer pprof.StopCPUProfile()
	//		time.Sleep(10 * time.Second)
	//	}()
	// ===== Profiling code =====

	config := srv.replConfig
	seqno := uint64(0)
	for {
		op, ok := <-config.ops
		if !ok {
			return
		}
		if wrap, err := unmarshalFcall(op.frame, op.wrapped); err != nil {
			log.Printf("Server %v: relayWriter unmarshal error: %v", srv.addr, err)
			// TODO: enqueue op with empty reply
		} else {
			// If this was a duplicate, we still need to ack with something that has
			// the same seqno
			if op.reply == nil {
				op.reply = wrap.Fcall
			}
			db.DLPrintf("RSRV", "%v Handling relay request %v", config.RelayAddr, wrap)
			var msg *RelayMsg
			// If we have never seen this request, process it.
			if wrap.Seqno == NO_SEQNO || seqno < wrap.Seqno {
				// Increment the sequence number
				seqno = seqno + 1
				fcall := wrap.Fcall
				db.DLPrintf("RSRV", " %v Reader sv req: %v\n", config.RelayAddr, fcall)
				// Serve the op first.
				reply := op.r.serve(fcall)
				srv.replyCache.Put(fcall)
				op.seqno = seqno
				db.DLPrintf("RSRV", "%v Reader rep: %v\n", config.RelayAddr, reply)
				// Store the reply
				op.reply = reply
				// Optionally log the fcall & its reply type.
				srv.logOp(fcall, reply)
				// Reliably send to the next link in the chain (even if that link
				// changes)
				if !srv.isTail() {
					// Only increment the seqno if this is a request from another replica
					msg = &RelayMsg{op, fcall, seqno}
					// Enqueue the message to mark it as in-flight.
					config.q.Enqueue(msg)
					srv.sendRelayMsg(msg)
				}
			} else {
				db.DLPrintf("RSRV", "%v Duplicate seqno: %v < %v", config.RelayAddr, wrap.Seqno, seqno)
				// We have already seen this request, and we aren't the tail. 2 Options:
				// 1. If it's in-flight (we haven't seen an ack from the tail yet), we
				//    need to register the op in order to have our ack thread send a
				//    reply.
				// 2. If it isn't in flight (we have already seen an ack from the tail
				//    for this request), we need to respond immediately with a dummy
				//    response.
				// Note that we do *not* need to resend the request, as our reliable
				// send mechanism will take care of this (and on configuration switch,
				// the request will be resent automatically).
				//
				// EnqueueDuplicate is a CAS-like function which atomically checks if
				// the op is in-flight, and if so, enqueues this duplicate and returns
				// true. Otherwise, it returns false. This atomicity is needed to make
				// sure we never drop acks which should be relayed upstream.
				// TODO: what about the tail? We shouldn't enqueue in this case, right?
				op.seqno = wrap.Seqno
				msg = &RelayMsg{op, wrap.Fcall, wrap.Seqno}
				if !config.q.EnqueueIfDuplicate(msg) && !srv.isTail() {
					db.DLPrintf("RSRV", "%v Didn't enqueue duplicate seqno: %v < %v", config.RelayAddr, wrap.Seqno, seqno)
					// TODO: send an empty reply, but this works for now since it
					// preserves the seqno.
					op.replies <- op
					continue
				} else {
					db.DLPrintf("RSRV", "%v Enqueued duplicate seqno: %v < %v", config.RelayAddr, wrap.Seqno, seqno)
				}
			}
			// If we're the tail, we always ack immediately
			if srv.isTail() {
				db.DLPrintf("RSRV", "%v Tail acking %v", config.RelayAddr, wrap.Fcall)
				op.replies <- op
			}
		}
	}
}

// Relay acks upstream.
func (srv *NpServer) relayWriter() {
	config := srv.replConfig
	for {
		// XXX Don't spin
		if srv.isTail() {
			continue
		}
		config.mu.Lock()
		ch := config.NextChan
		config.mu.Unlock()
		// Channels start as nil during initialization.
		if ch == nil {
			continue
		}
		// Get an ack from the downstream servers
		frame, err := ch.Recv()
		// Move on if the connection closed
		if peerCrashed(err) {
			db.DLPrintf("RSRV", "%v error relayWriter Recv: %v", config.RelayAddr, err)
			continue
		}
		if err != nil {
			log.Printf("%v error receiving ack: %v\n", config.RelayAddr, err)
			continue
		}
		wrap, err := unmarshalFcall(frame, true)
		if err != nil {
			log.Printf("Error unmarshalling in relayWriter: %v", err)
		}
		db.DLPrintf("RSRV", "%v Got ack: %v", config.RelayAddr, wrap)
		// Dequeue all acks up until this one (they may come out of order, which is
		// OK.
		msgs := config.q.DequeueUntil(wrap.Seqno)
		db.DLPrintf("RSRV", "%v Dequeued until: %v", config.RelayAddr, msgs)
		// Ack upstream
		for _, msg := range msgs {
			msg.op.replies <- msg.op
		}
	}
}

func (srv *NpServer) sendRelayMsg(msg *RelayMsg) {
	config := srv.replConfig

	// Get the next channel & address of the last person we sent to...
	config.mu.Lock()
	ch := config.NextChan
	nextAddr := config.NextAddr
	lastSendAddr := config.LastSendAddr
	config.mu.Unlock()

	// If the next server has changed (detected by config swap, or message send
	// failure), resend all in-flight requests. Should already include this
	// message.
	db.DLPrintf("RSRV", "%v -> %v Sending initial relayMsg: %v", config.RelayAddr, nextAddr, msg)
	if lastSendAddr != nextAddr || !srv.relayOnce(ch, msg) {
		srv.resendInflightRelayMsgs()
	}
}

func (srv *NpServer) resendInflightRelayMsgs() {
	config := srv.replConfig

	// Get the connection to the next server, and reflect that we've sent to it.
	config.mu.Lock()
	ch := config.NextChan
	config.LastSendAddr = config.NextAddr
	nextAddr := config.NextAddr
	config.mu.Unlock()

	toSend := config.q.GetQ()
	db.DLPrintf("RSRV", "%v -> %v Resending inflight messages: %v", config.RelayAddr, nextAddr, toSend)
	// Retry. On failure, resend all messages which haven't been ack'd, plus msg.
	for len(toSend) != 0 {
		// We shouldn't send anything if we're the tail
		if srv.isTail() {
			db.DLPrintf("RSRV", "%v -> %v Was tail, cancelling resend", config.RelayAddr, nextAddr)
			return
		}
		// Try to send a message.
		if ok := srv.relayOnce(ch, toSend[0]); ok {
			// If successful, move on to the next one
			toSend = toSend[1:]
		} else {
			// Else, retry sending the whole queue again
			config.mu.Lock()
			ch = config.NextChan
			config.LastSendAddr = config.NextAddr
			config.mu.Unlock()
			toSend = config.q.GetQ()
			db.DLPrintf("RSRV", "%v -> %v Resending inflight messages: %v", config.RelayAddr, nextAddr, toSend)
		}
	}
	db.DLPrintf("RSRV", "%v Done Resending inflight messages to %v", config.RelayAddr, nextAddr)
}

func (srv *NpServer) sendAllAcks() {
	config := srv.replConfig
	msgs := config.q.DequeueUntil(math.MaxUint64)
	db.DLPrintf("RSRV", "%v Sent all acks: %v", config.RelayAddr, msgs)
	// Ack upstream
	go func() {
		for _, msg := range msgs {
			msg.op.replies <- msg.op
		}
	}()
}

// Try and send a message to the next server in the chain, and receive a
// response.
func (srv *NpServer) relayOnce(ch *RelayConn, msg *RelayMsg) bool {
	// Only call down the chain if we aren't at the tail.
	// XXX Get rid of this if
	if !srv.isTail() {
		var err error
		if msg.op.wrapped {
			// Just pass wrapped op along...
			err = ch.Send(msg.op.frame)
		} else {
			// If this op hasn't been wrapped, wrap it before we send it.
			var frame []byte
			frame, err = marshalFcall(msg.fcall, true, msg.seqno)
			if err != nil {
				log.Fatalf("Error marshalling fcall: %v", err)
			}
			err = ch.Send(frame)
		}
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
func (srv *NpServer) logOp(fcall *np.Fcall, reply *np.Fcall) {
	config := srv.replConfig
	if config.LogOps {
		fpath := "name/" + config.RelayAddr + "-log.txt"
		b, err := config.ReadFile(fpath)
		if err != nil {
			log.Printf("Error reading log file in logOp: %v", err)
		}
		frame, err := npcodec.Marshal(fcall)
		if err != nil {
			log.Printf("Error marshalling fcall in logOp: %v", err)
		}
		b = append(b, frame...)
		b = append(b, []byte(reply.Type.String())...)
		err = config.WriteFile(fpath, b)
		if err != nil {
			log.Printf("Error writing log file in logOp: %v", err)
		}
	}
}

func (w *FcallWrapper) String() string {
	return fmt.Sprintf("{ seqno:%v fcall:%v }", w.Seqno, w.Fcall)
}

func (op *SrvOp) String() string {
	return fmt.Sprintf("{ seqno:%v wrapped:%v reply:%v }", op.seqno, op.wrapped, op.reply)
}

func peerCrashed(err error) bool {
	return err != nil &&
		(err.Error() == "EOF" ||
			strings.Contains(err.Error(), "use of closed network connection") ||
			strings.Contains(err.Error(), "connection reset by peer"))
}
