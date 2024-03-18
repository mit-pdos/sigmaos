// The demux package multiplexes calls over a transport (e.g., TCP
// connection, unix socket, etc.), and matches responses with requests
// using the call's tag.
package demux

import (
	"sync"

	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
)

type DemuxClnt struct {
	callmap *callMap
	trans   TransportI
	iovm    *IoVecMap
	mu      sync.Mutex
}

type reply struct {
	rep CallI
	err *serr.Err
}

func NewDemuxClnt(trans TransportI, iovm *IoVecMap) *DemuxClnt {
	dmx := &DemuxClnt{
		callmap: newCallMap(),
		trans:   trans,
		iovm:    iovm,
	}
	go dmx.reader()
	return dmx
}

func (dmx *DemuxClnt) reply(tag sessp.Ttag, rep CallI, err *serr.Err) {
	if ch, ok := dmx.callmap.remove(tag); ok {
		ch <- reply{rep, err}
	} else {
		db.DFatalf("reply %v no matching req %v\n", rep, tag)
	}
}

func (dmx *DemuxClnt) reader() {
	for {
		c, err := dmx.trans.ReadCall()
		if err != nil {
			db.DPrintf(db.DEMUXCLNT, "reader rf err %v\n", err)
			dmx.callmap.close()
			break
		}
		dmx.reply(c.Tag(), c, nil)
	}
	for _, t := range dmx.callmap.outstanding() {
		db.DPrintf(db.DEMUXCLNT, "reader fail %v\n", t)
		dmx.reply(t, nil, serr.NewErr(serr.TErrUnreachable, "reader"))
	}
}

func (dmx *DemuxClnt) SendReceive(req CallI, outiov sessp.IoVec) (CallI, *serr.Err) {
	ch := make(chan reply)
	if err := dmx.callmap.put(req.Tag(), ch); err != nil {
		db.DPrintf(db.DEMUXCLNT, "SendReceive: enqueue req %v err %v\n", req, err)
		return nil, err
	}
	db.DPrintf(db.ALWAYS, "demuxclnt iov len %v", len(outiov))
	if len(outiov) > 0 {
		db.DPrintf(db.ALWAYS, "demuxclnt iov len %v p %p", len(outiov), outiov[len(outiov)-1])
	}
	if false {
		db.DPrintf(db.ALWAYS, "Put %v outiovlen %v\nstack %v", req, len(outiov), string(debug.Stack()))
	}
	if err := dmx.iovm.Put(req.Tag(), outiov); err != nil {
		db.DPrintf(db.DEMUXCLNT, "SendReceive: iovm enqueue req %v err %v\n", req, err)
		return nil, err
	}
	dmx.mu.Lock()
	err := dmx.trans.WriteCall(req)
	dmx.mu.Unlock()
	if err != nil {
		db.DPrintf(db.DEMUXCLNT, "wf req %v error %v\n", req, err)
		return nil, err
	}
	rep := <-ch
	return rep.rep, rep.err
}

func (dmx *DemuxClnt) Close() error {
	return dmx.callmap.close()
}

func (dmx *DemuxClnt) IsClosed() bool {
	return dmx.callmap.isClosed()
}
