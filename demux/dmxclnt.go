package demux

import (
	"bufio"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/sessp"
	// sp "sigmaos/sigmap"
	"sigmaos/serr"
)

// DemuxClnt multiplexes RPCs on a single transport and
// demultiplexes responses.
type DemuxClnt struct {
	out    *bufio.Writer
	in     *bufio.Reader
	seqno  sessp.Tseqno
	rpcmap *rpcMap
	rpcs   chan *rpc
}

func NewDemuxClnt(out *bufio.Writer, in *bufio.Reader) *DemuxClnt {
	dmx := &DemuxClnt{out, in, 0, newRpcMap(), make(chan *rpc)}
	go dmx.reader()
	go dmx.writer()
	return dmx
}

func (dmx *DemuxClnt) writer() {
	for {
		rpc, ok := <-dmx.rpcs
		if !ok {
			db.DPrintf(db.DEMUXCLNT, "writer: replies closed\n")
			return
		}
		if err := frame.WriteSeqno(rpc.seqno, dmx.out); err != nil {
			db.DPrintf(db.DEMUXCLNT, "WriteSeqno err %v\n", err)
			dmx.reply(rpc.seqno, nil, serr.NewErr(serr.TErrUnreachable, err.Error()))
			break
		}
		if err := frame.WriteFrame(dmx.out, rpc.request); err != nil {
			db.DPrintf(db.DEMUXCLNT, "WriteFrame err %v\n", err)
			dmx.reply(rpc.seqno, nil, serr.NewErr(serr.TErrUnreachable, err.Error()))
			break
		}
		if error := dmx.out.Flush(); error != nil {
			db.DPrintf(db.DEMUXCLNT, "Flush error %v\n", error)
			dmx.reply(rpc.seqno, nil, serr.NewErr(serr.TErrUnreachable, error.Error()))
		}
	}
}

func (dmx *DemuxClnt) reply(seqno sessp.Tseqno, reply []byte, err error) {
	rpc, last := dmx.rpcmap.remove(seqno)
	if rpc == nil {
		db.DFatalf("Remove err %v\n", seqno)
	}
	if last {
		close(dmx.rpcs)
	}
	rpc.reply = reply
	rpc.ch <- err
}

func (dmx *DemuxClnt) reader() {
	for {
		seqno, err := frame.ReadSeqno(dmx.in)
		if err != nil {
			db.DPrintf(db.DEMUXCLNT, "reader: ReadSeqno err %v\n", err)
			break
		}
		reply, err := frame.ReadFrame(dmx.in)
		if err != nil {
			db.DPrintf(db.DEMUXCLNT, "reader: ReadFrame err %v\n", err)
			break
		}
		db.DPrintf(db.DEMUXCLNT, "reader: reply %v\n", seqno)
		dmx.reply(seqno, reply, nil)
	}
	for _, s := range dmx.rpcmap.outstanding() {
		dmx.reply(s, nil, serr.NewErr(serr.TErrUnreachable, "dmxclnt"))
	}
}

func (dmx *DemuxClnt) SendReceive(a []byte) ([]byte, error) {
	seqp := &dmx.seqno
	s := seqp.Next()
	rpc := &rpc{request: a, seqno: s, ch: make(chan error)}
	if err := dmx.rpcmap.put(s, rpc); err != nil {
		db.DPrintf(db.DEMUXCLNT, "SendReceive: enqueue err %v\n", err)
		return nil, err
	}
	db.DPrintf(db.DEMUXCLNT, "SendReceive: enqueue %v\n", rpc)
	dmx.rpcs <- rpc
	err := <-rpc.ch
	db.DPrintf(db.DEMUXCLNT, "SendReceive: return %v %v\n", rpc, err)
	return rpc.reply, err
}

func (dmx *DemuxClnt) Close() error {
	dmx.rpcmap.close()
	return nil
}
