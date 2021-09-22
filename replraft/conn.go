package replraft

import (
	"bufio"
	"io"
	"log"
	"net"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/protsrv"
)

const (
	Msglen = 64 * 1024
)

type RaftReplConn struct {
	clerk   *Clerk
	fssrv   protsrv.FsServer
	np      protsrv.Protsrv
	conn    net.Conn
	br      *bufio.Reader
	bw      *bufio.Writer
	replies chan *SrvOp
}

type SrvOp struct {
	request *np.Fcall
	frame   []byte
	reply   *np.Fcall
	replyC  chan *SrvOp
}

func MakeRaftReplConn(psrv protsrv.FsServer, conn net.Conn, clerk *Clerk) *RaftReplConn {
	protsrv := psrv.Connect()
	r := &RaftReplConn{
		clerk,
		psrv, protsrv, conn,
		bufio.NewReaderSize(conn, Msglen),
		bufio.NewWriterSize(conn, Msglen),
		make(chan *SrvOp)}
	go r.reader()
	go r.writer()
	return r
}

func (c *RaftReplConn) reader() {
	db.DLPrintf("REPLRAFT", "Conn from %v\n", c.Src())
	for {
		frame, err := npcodec.ReadFrame(c.br)
		if err != nil {
			db.DLPrintf("REPLRAFT", "%v Peer %v closed/erred %v\n", c.Dst(), c.Src(), err)
			if err == io.EOF {
				c.close()
			}
			return
		}
		db.DLPrintf("REPLRAFT", "%v raft reader read frame from %v\n", c.Dst(), c.Src())
		fcall := &np.Fcall{}
		if err := npcodec.Unmarshal(frame, fcall); err != nil {
			log.Printf("Server %v: replraft reader unmarshal error: %v", c.Dst(), err)
		} else {
			op := &SrvOp{fcall, frame, nil, c.replies}
			db.DLPrintf("REPLRAFT", "%v raft about to request %v clerk %v\n", c.Dst(), fcall, c.Src())
			c.clerk.request(op)
			db.DLPrintf("REPLRAFT", "%v raft reader requested from clerk %v\n", c.Dst(), c.Src())
		}
	}
}

func (c *RaftReplConn) writer() {
	for {
		op, ok := <-c.replies
		if !ok {
			return
		}
		db.DLPrintf("REPLRAFT", "%v -> %v raft writer reply: %v", c.Dst(), c.Src(), op.reply)
		err := npcodec.MarshalFcallToWriter(op.reply, c.bw)
		if err != nil {
			db.DLPrintf("REPLRAFT", "%v -> %v Writer: WriteFrame error: %v", c.Src(), c.Dst(), err)
			continue
		}
		err = c.bw.Flush()
		if err != nil {
			db.DLPrintf("REPLRAFT", "%v -> %v Writer: Flush error: %v", c.Src(), c.Dst(), err)
			continue
		}
	}
}

func (c *RaftReplConn) Src() string {
	return c.conn.RemoteAddr().String()
}

func (c *RaftReplConn) Dst() string {
	return c.conn.LocalAddr().String()
}

func (c *RaftReplConn) close() {
	db.DLPrintf("REPLRAFT", "Close: %v", c.Src())
}
