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
	NO_SEQNO = 0
)

type FcallWrapper struct {
	Seqno uint64
	Fcall *np.Fcall
}

type RelayOp struct {
	wrapped bool
	frame   []byte
	r       *Relay
	replies chan []byte
}

// XXX when do I close the replies channel? the ops channel?
type Relay struct {
	c       *Channel
	ops     chan *RelayOp
	replies chan []byte
	wrapped bool
}

func MakeRelay(npc NpConn, conn net.Conn, ops chan *RelayOp, wrapped bool) *Relay {
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
	db.DLPrintf("9PCHAN", "Reader conn from %v\n", r.c.Src())
	for {
		frame, err := npcodec.ReadFrame(r.c.br)
		if err != nil {
			db.DLPrintf("9PCHAN", "Peer %v closed/erred %v\n", r.c.Src(), err)
			if err == io.EOF {
				r.c.close()
			}
			return
		}
		op := &RelayOp{r.wrapped, frame, r, r.replies}
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
			log.Print("Writer: WriteFrame error ", err)
			return
		}
		err = r.c.bw.Flush()
		if err != nil {
			log.Print("Writer: Flush error ", err)
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
	for {
		op, ok := <-config.ops
		if !ok {
			return
		}
		if wrap, err := unmarshalFcall(op.frame, op.wrapped); err != nil {
			log.Printf("Server %v: relayChanWorker unmarshal error: %v", srv.addr, err)
		} else {
			fcall := wrap.Fcall
			db.DLPrintf("9PCHAN", "Reader sv req: %v\n", fcall)
			// Serve the op first.
			reply := op.r.serve(fcall)
			// Only call down the chain if we aren't at the tail.
			if !srv.isTail() {
				// XXX Have to think carefully about how we want to "Call" the next server...
				var err error
				if op.wrapped {
					// Just pass wrapped op along...
					err = config.NextChan.Send(op.frame)
				} else {
					// If this op hasn't been wrapped, wrap it before we send it.
					var frame []byte
					// TODO: Set seqno appropriately
					frame, err = marshalFcall(fcall, true, 0)
					if err != nil {
						log.Printf("Error marshalling fcall: %v", err)
					}
					err = config.NextChan.Send(frame)
					if err != nil {
						log.Printf("Error sending wrapped fcall: %v", err)
					}
				}
				// If the next server has crashed...
				if err != nil && err.Error() == "EOF" {
					// TODO: on crash...
					// 1. Switch to the next config.
					// 2. If I'm still a middle server, then change relay & send to the
					// next server.
					// 3. If I'm now the head server, propagate the call.
					//   XXX may err on  response...
					// 4. If I'm now the tail server, then serve & return
					log.Printf("Srv sending error: %v", err)
				}
				if err != nil {
					log.Printf("Error sending: %v\n", err)
				}
				_, err = config.NextChan.Recv()
				if err != nil {
					log.Printf("Error receiving: %v\n", err)
				}
				// TODO: bookkeeping marking as received
			}
			// Send responpse back to client
			db.DLPrintf("9PCHAN", "Writer rep: %v\n", reply)
			frame, err := marshalFcall(reply, op.wrapped, 0)
			if err != nil {
				log.Print("Writer: marshal error: ", err)
			} else {
				op.replies <- frame
			}
		}
	}
}
