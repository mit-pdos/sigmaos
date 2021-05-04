package npsrv

import (
	"bufio"
	"io"
	"log"
	"net"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

type RelayOp struct {
	frame   []byte
	rc      *RelayChannel
	replies chan []byte
}

// XXX when do I close the replies channel? the ops channel?
type RelayChannel struct {
	c       *Channel
	ops     chan *RelayOp
	replies chan []byte
}

func MakeRelayChannel(npc NpConn, conn net.Conn, ops chan *RelayOp) *RelayChannel {
	npapi := npc.Connect(conn)
	c := &Channel{npc,
		conn,
		npapi,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		false,
	}
	rc := &RelayChannel{c, ops, make(chan []byte)}
	go rc.writer()
	go rc.reader()
	return rc
}

func (rc *RelayChannel) reader() {
	db.DLPrintf("9PCHAN", "Reader conn from %v\n", rc.c.Src())
	for {
		frame, err := npcodec.ReadFrame(rc.c.br)
		if err != nil {
			db.DLPrintf("9PCHAN", "Peer %v closed/erred %v\n", rc.c.Src(), err)
			if err == io.EOF {
				rc.c.close()
			}
			return
		}
		op := &RelayOp{frame, rc, rc.replies}
		rc.ops <- op
	}
}

func (rc *RelayChannel) serve(fc *np.Fcall) *np.Fcall {
	t := fc.Tag
	reply, rerror := rc.c.dispatch(fc.Msg)
	if rerror != nil {
		reply = *rerror
	}
	fcall := &np.Fcall{}
	fcall.Type = reply.Type()
	fcall.Msg = reply
	fcall.Tag = t
	return fcall
}

func (rc *RelayChannel) writer() {
	for {
		frame, ok := <-rc.replies
		if !ok {
			return
		}
		err := npcodec.WriteFrame(rc.c.bw, frame)
		if err != nil {
			log.Print("Writer: WriteFrame error ", err)
			return
		}
		err = rc.c.bw.Flush()
		if err != nil {
			log.Print("Writer: Flush error ", err)
			return
		}
	}
}

func (srv *NpServer) relayWorker() {
	config := srv.replConfig
	for {
		op, ok := <-config.ops
		if !ok {
			return
		}
		fcall := &np.Fcall{}
		if err := npcodec.Unmarshal(op.frame, fcall); err != nil {
			log.Print("Serve: unmarshal error: ", err)
		} else {
			db.DLPrintf("9PCHAN", "Reader sv req: %v\n", fcall)
			// Serve the op first.
			reply := op.rc.serve(fcall)
			// Only call down the chain if we aren't at the tail.
			if !srv.isTail() {
				_, err := config.NextChan.Call(fcall.Msg)
				// If the next server has crashed...
				if err != nil && err.Error() == "EOF" {
					// TODO:
					// 1. Switch to the next config.
					// 2. If I'm still a middle server, then change relay & send to the
					// next server.
					// 3. If I'm now the head server, propagate the call.
					//   XXX may err on  response...
					// 4. If I'm now the tail server, then serve & return
					log.Printf("Srv reader error: %v", err)
				}
			}
			db.DLPrintf("9PCHAN", "Writer rep: %v\n", reply)
			frame, err := npcodec.Marshal(reply)
			if err != nil {
				log.Print("Writer: marshal error: ", err)
			} else {
				op.replies <- frame
			}
		}
	}
}
