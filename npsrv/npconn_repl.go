package npsrv

import (
	"bufio"
	"io"
	"log"
	"net"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npclnt"
	"ulambda/npcodec"
)

type RelayChannel struct {
	c      *Channel
	relay  *npclnt.NpChan
	isTail bool
}

func MakeRelayChannel(npc NpConn, conn net.Conn, relay *npclnt.NpChan, isTail bool) *RelayChannel {
	npapi := npc.Connect(conn)
	c := &Channel{npc,
		conn,
		npapi,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *np.Fcall),
		false,
	}
	rc := &RelayChannel{c, relay, isTail}
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
		// XXX This work can and should be done in another thread...
		fcall := &np.Fcall{}
		if err := npcodec.Unmarshal(frame, fcall); err != nil {
			log.Print("Serve: unmarshal error: ", err)
		} else {
			// Only call down the chain if we aren't at the tail.
			if !rc.isTail {
				_, err := rc.relay.Call(fcall.Msg)
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
			db.DLPrintf("9PCHAN", "Reader sv req: %v\n", fcall)
			// XXX Should I serve before calling to replicas?
			rc.serve(fcall)
		}
	}
}

func (rc *RelayChannel) serve(fc *np.Fcall) {
	t := fc.Tag
	reply, rerror := rc.c.dispatch(fc.Msg)
	if rerror != nil {
		reply = *rerror
	}
	fcall := &np.Fcall{}
	fcall.Type = reply.Type()
	fcall.Msg = reply
	fcall.Tag = t
	if !rc.c.closed {
		rc.c.replies <- fcall
	}
}

func (rc *RelayChannel) writer() {
	for {
		fcall, ok := <-rc.c.replies
		if !ok {
			return
		}
		db.DLPrintf("9PCHAN", "Writer rep: %v\n", fcall)
		frame, err := npcodec.Marshal(fcall)
		if err != nil {
			log.Print("Writer: marshal error: ", err)
		} else {
			// log.Print("Srv: Rframe ", len(frame), frame)
			err = npcodec.WriteFrame(rc.c.bw, frame)
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
}
