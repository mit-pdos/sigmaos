// The demux package multiplexes calls over a transport (e.g., TCP
// connection, unix socket, etc.), and matches responses with requests
// using the call's tag.
package demux

import (
	"sync"

	"reflect"
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
	db.DPrintf(db.DEMUXCLNT_ERR, "new demuxClnt")
	printStructFields(*dmx)
	return dmx
}

func (dmx *DemuxClnt) reply(tag sessp.Ttag, rep CallI, err *serr.Err) {
	if ch, ok := dmx.callmap.remove(tag); ok {
		ch <- reply{rep, err}
	} else {
		db.DPrintf(db.ERROR, "reply %v no matching req %v", rep, tag)
	}
}

func printStructFields(s interface{}) {
	// Get the type and value of the struct
	val := reflect.ValueOf(s)
	typ := reflect.TypeOf(s)

	// Ensure the input is a struct
	if typ.Kind() != reflect.Struct {
		db.DPrintf(db.DEMUXCLNT_ERR, "Provided input is not a struct %v", typ.Kind())
		return
	}

	// Iterate through the fields of the struct
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		value := val.Field(i)

		db.DPrintf(db.DEMUXCLNT_ERR, "Field Name: %s, Field Type: %s, Field Value: %v\n", field.Name, field.Type, value)
	}
}
func (dmx *DemuxClnt) reader() {
	for {
		c, err := dmx.trans.ReadCall()
		if err != nil {
			db.DPrintf(db.DEMUXCLNT_ERR, "reader rf err %v", err)
			printStructFields(*dmx)
			dmx.callmap.close()
			break
		}
		dmx.reply(c.Tag(), c, nil)
	}
	outstanding := dmx.callmap.outstanding()
	db.DPrintf(db.DEMUXCLNT_ERR, "reader fail oustanding %v", outstanding)
	for _, t := range outstanding {
		db.DPrintf(db.DEMUXCLNT_ERR, "reader reply fail %v", t)
		dmx.reply(t, nil, serr.NewErr(serr.TErrUnreachable, "reader"))
		db.DPrintf(db.DEMUXCLNT_ERR, "reader reply fail done %v", t)
	}
}

func (dmx *DemuxClnt) SendReceive(req CallI, outiov sessp.IoVec) (CallI, *serr.Err) {
	ch := make(chan reply)
	if err := dmx.callmap.put(req.Tag(), ch); err != nil {
		db.DPrintf(db.DEMUXCLNT_ERR, "SendReceive: enqueue req %v err %v", req, err)
		printStructFields(*dmx)
		return nil, err
	}
	if err := dmx.iovm.Put(req.Tag(), outiov); err != nil {
		db.DPrintf(db.DEMUXCLNT_ERR, "SendReceive: iovm enqueue req %v err %v", req, err)
		return nil, err
	}
	dmx.mu.Lock()
	err := dmx.trans.WriteCall(req)
	dmx.mu.Unlock()
	if err != nil {
		db.DPrintf(db.DEMUXCLNT_ERR, "WriteCall req %v error %v", req, err)
	}
	// Listen to the reply channel regardless of error status, so the reader
	// thread doesn't block indefinitely trying to deliver the "TErrUnreachable"
	// reply.
	rep := <-ch
	return rep.rep, rep.err
}

func (dmx *DemuxClnt) Close() error {
	db.DPrintf(db.DEMUXCLNT_ERR, "closing")
	return dmx.callmap.close()
}

func (dmx *DemuxClnt) IsClosed() bool {
	return dmx.callmap.isClosed()
}
