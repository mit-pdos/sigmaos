package npsrv

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"io"
	"log"
	"net"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
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
	frame   []byte
	reply   []byte
	r       *RelayChannel
	replies chan []byte
}

// A channel between clients & replicas, or replicas & replicas
type RelayChannel struct {
	c       *Channel
	ops     chan *SrvOp
	replies chan []byte
	wrapped bool
}

func MakeRelayChannel(npc NpConn, conn net.Conn, ops chan *SrvOp, wrapped bool) *RelayChannel {
	npapi := npc.Connect(conn)
	c := &Channel{npc,
		conn,
		npapi,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		false,
	}
	r := &RelayChannel{c, ops, make(chan []byte), wrapped}
	go r.writer()
	go r.reader()
	return r
}

func (r *RelayChannel) reader() {
	db.DLPrintf("RSRV", "Conn from %v\n", r.c.Src())
	for {
		frame, err := npcodec.ReadFrame(r.c.br)
		if err != nil {
			db.DLPrintf("RSRV", "Peer %v closed/erred %v\n", r.c.Src(), err)
			if err == io.EOF {
				r.c.close()
			}
			return
		}
		op := &SrvOp{r.wrapped, frame, []byte{}, r, r.replies}
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
		frame, ok := <-r.replies
		if !ok {
			return
		}
		err := npcodec.WriteFrame(r.c.bw, frame)
		if err != nil {
			log.Printf("Writer: WriteFrame error: %v", err)
			return
		}
		err = r.c.bw.Flush()
		if err != nil {
			log.Printf("Writer: Flush error: %v", err)
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
			db.DLPrintf("RSRV", "Relay request: %v\n", wrap)
			// If we have never seen this request, process it.
			if wrap.Seqno == NO_SEQNO || seqno < wrap.Seqno {
				// Increment the sequence number
				seqno = seqno + 1
				fcall := wrap.Fcall
				db.DLPrintf("RSRV", "Reader sv req: %v\n", fcall)
				// Serve the op first.
				reply := op.r.serve(fcall)
				db.DLPrintf("RSRV", "Reader rep: %v\n", reply)
				frame, err := marshalFcall(reply, op.wrapped, wrap.Seqno)
				if err != nil {
					log.Fatalf("Writer: marshal error: %v", err)
				}
				// Store the reply
				op.reply = frame
				// Optionally log the fcall.
				srv.logOp(fcall)
				// Reliably send to the next link in the chain (even if that link
				// changes)
				if !srv.isTail() {
					// TODO: fix how we reliably send
					// Only increment the seqno if this is a request from another replica
					msg := &RelayMsg{op, fcall, seqno}
					srv.relayReliable(msg)
				}
			} else {
				// We have already seen this request. 2 Options:
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
				msg := &RelayMsg{op, wrap.Fcall, wrap.Seqno}
				if !config.q.EnqueueDuplicate(msg) {
					log.Printf("Ignoring duplicate seqno: %v < %v", wrap.Seqno, seqno)
					// TODO: send an empty reply, but this works for now since it
					// preserves the seqno.
					op.replies <- op.frame
					continue
				}
			}
			// TODO: If we're the tail, we always ack immediately
		}
	}
}

func (srv *NpServer) relayWriter() {
	config := srv.replConfig
	for {
		// Get an ack from the downstream servers
		frame, err := config.NextChan.Recv()
		if err != nil {
			log.Printf("Srv error receiving: %v\n", err)
		}
		wrap, err := unmarshalFcall(frame, true)
		if err != nil {
			log.Printf("Error unmarshalling in relayWriter: %v", err)
		}
		// Dequeue all acks up until this one
		msgs := config.q.DequeueUntil(wrap.Seqno)
		// Ack upstream
		for _, msg := range msgs {
			msg.op.replies <- msg.op.reply
		}
	}
}

func (srv *NpServer) relayReliable(msg *RelayMsg) {
	config := srv.replConfig
	toSend := []*RelayMsg{msg}
	// Retry. On failure, resend all messages which haven't been ack'd, plus msg.
	for len(toSend) != 0 {
		// We don't want to switch which server we're sending to mid-stream.
		config.mu.Lock()
		// Try to send a message.
		if ok := srv.relayOnce(config, toSend[0]); ok {
			// If successful, move on to the next one
			toSend = toSend[1:]
		} else {
			// If unsuccessful, restart with queue of un-acked messages + msg.
			toSend = append(config.q.GetQ(), msg)
		}
		config.mu.Unlock()
	}
}

// Try and send a message to the next server in the chain, and receive a
// response.
func (srv *NpServer) relayOnce(config *NpServerReplConfig, msg *RelayMsg) bool {
	// Only call down the chain if we aren't at the tail.
	if !srv.isTail() {
		var err error
		if msg.op.wrapped {
			// Just pass wrapped op along...
			err = config.NextChan.Send(msg.op.frame)
		} else {
			// If this op hasn't been wrapped, wrap it before we send it.
			var frame []byte
			frame, err = marshalFcall(msg.fcall, true, msg.seqno)
			if err != nil {
				log.Fatalf("Error marshalling fcall: %v", err)
			}
			err = config.NextChan.Send(frame)
		}
		// If the next server has crashed, retry...
		if err != nil && err.Error() == "EOF" {
			log.Printf("Srv sending error: %v", err)
			return false
		}
		if err != nil {
			log.Fatalf("Srv error sending: %v\n", err)
		}
		// If send was successful, enqueue the relayMsg until ack'd.
		config.q.Enqueue(msg)
	}
	// If we made it this far, exit
	return true
}

func (srv *NpServer) logOp(fcall *np.Fcall) {
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
		err = config.WriteFile(fpath, b)
		if err != nil {
			log.Printf("Error writing log file in logOp: %v", err)
		}
	}
}
