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
	r       *Relay
	replies chan []byte
}

// XXX when do I close the replies channel? the ops channel?
type Relay struct {
	c       *Channel
	ops     chan *SrvOp
	replies chan []byte
	wrapped bool
}

func MakeRelay(npc NpConn, conn net.Conn, ops chan *SrvOp, wrapped bool) *Relay {
	npapi := npc.Connect(conn)
	c := &Channel{npc,
		conn,
		npapi,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		false,
	}
	r := &Relay{c, ops, make(chan []byte), wrapped}
	go r.writer()
	go r.reader()
	return r
}

func (r *Relay) reader() {
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
		op := &SrvOp{r.wrapped, frame, r, r.replies}
		r.ops <- op
	}
}

func (r *Relay) serve(fc *np.Fcall) *np.Fcall {
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

func (r *Relay) writer() {
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

func (srv *NpServer) relayChanWorker() {
	config := srv.replConfig
	seqno := uint64(0)
	for {
		op, ok := <-config.ops
		if !ok {
			return
		}
		if wrap, err := unmarshalFcall(op.frame, op.wrapped); err != nil {
			log.Printf("Server %v: relayChanWorker unmarshal error: %v", srv.addr, err)
		} else {
			// If we have already seen this request, don't process it. Send a dummy
			// response (ok since responses are ignored anyway).
			if seqno >= wrap.Seqno && wrap.Seqno != NO_SEQNO {
				log.Printf("Ignoring duplicate seqno: %v < %v", wrap.Seqno, seqno)
				op.replies <- []byte(DUMMY_RESPONSE)
				continue
			}
			fcall := wrap.Fcall
			db.DLPrintf("RSRV", "Reader sv req: %v\n", fcall)
			// Serve the op first.
			reply := op.r.serve(fcall)
			// Optionally log the fcall.
			srv.logOp(fcall)
			// Reliably send to the next link in the chain (even if that link changes)
			if !srv.isTail() {
				// Only increment the seqno if this is a request from another replica
				if wrap.Seqno != NO_SEQNO {
					seqno = seqno + 1
				}
				msg := &RelayMsg{op, fcall, seqno}
				srv.relayReliable(msg)
			}
			// Send responpse back to client
			db.DLPrintf("RSRV", "Writer rep: %v\n", reply)
			frame, err := marshalFcall(reply, op.wrapped, wrap.Seqno)
			if err != nil {
				log.Print("Writer: marshal error: ", err)
			} else {
				op.replies <- frame
			}
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
		// Start a thread to wait on the ack & update RelayMsgQueue
		go func(seqno uint64) {
			data, err := config.NextChan.Recv()
			if err != nil {
				log.Printf("Srv error receiving: %v\n", err)
			}
			config.q.DequeueUntil(seqno)
			if string(data) == DUMMY_RESPONSE {
				log.Printf("Dummy response in srv: %v", srv.addr)
			}
		}(msg.seqno)
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
