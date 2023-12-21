package demux

import (
	"io"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/serr"
)

type DemuxI interface {
	ServeRequest([]byte) ([]byte, *serr.Err)
}

type DemuxSrv struct {
	in      io.Reader
	out     io.Writer
	serve   DemuxI
	replies chan []byte
}

func NewDemuxSrv(in io.Reader, out io.Writer, serve DemuxI) *DemuxSrv {
	dmx := &DemuxSrv{in, out, serve, make(chan []byte)}
	go dmx.reader()
	go dmx.writer()
	return dmx
}

func (dmx *DemuxSrv) reader() {
	for {
		request, err := frame.ReadFrame(dmx.in)
		if err != nil {
			db.DPrintf(db.DEMUXSRV, "reader: ReadFrame err %v\n", err)
			break
		}
		go func(r []byte) {
			reply, err := dmx.serve.ServeRequest(r)
			if err != nil {
				db.DPrintf(db.DEMUXSRV, "reader: ServeRequest err %v\n", err)
				return
			}
			dmx.replies <- reply
		}(request)
	}
}

func (dmx *DemuxSrv) writer() {
	for {
		reply, ok := <-dmx.replies
		if !ok {
			db.DPrintf(db.DEMUXSRV, "%v writer: replies closed\n")
			return
		}
		if err := frame.WriteFrame(dmx.out, reply); err != nil {
			db.DFatalf("writer: writeFrame err %v\n", err)
		}
	}
}
