// Package rpcclnt is the client stub that (un)marshals argument
// protobufs and result protobufs.  By default it uses sigmaos to
// send/receive the request and receive reply.  But caller can pass
// into its own RPCCh interface to replace the default (e.g., to
// send/receive over a pipe).

package rpcclnt

import (
	"path"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/rpc"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	"sigmaos/sessdevclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type RPCcall struct {
	method string
	iov    sessp.IoVec
}

type RPCCh interface {
	SendReceive(sessp.IoVec) (sessp.IoVec, error)
	StatsSrv() (*rpc.SigmaRPCStats, error)
}

type RPCClnt struct {
	si *rpc.StatInfo
	ch RPCCh
}

func NewRPCClntCh(ch RPCCh) *RPCClnt {
	rpcc := &RPCClnt{
		si: rpc.NewStatInfo(),
		ch: ch,
	}
	return rpcc
}

type sigmaCh struct {
	fsls []*fslib.FsLib
	fds  []int
	pn   string
	idx  int32
}

func newSigmaCh(fsls []*fslib.FsLib, pn string) (RPCCh, error) {
	rpcch := &sigmaCh{
		fsls: make([]*fslib.FsLib, 0, len(fsls)),
		fds:  make([]int, 0, len(fsls)),
		pn:   pn,
	}
	sdc, err := sessdevclnt.NewSessDevClnt(fsls[0], path.Join(pn, rpc.RPC))
	if err != nil {
		return nil, err
	}
	for _, fsl := range fsls {
		rpcch.fsls = append(rpcch.fsls, fsl)
		fd, err := fsl.Open(sdc.DataPn(), sp.ORDWR)
		if err != nil {
			return nil, err
		}
		rpcch.fds = append(rpcch.fds, fd)
	}
	return rpcch, nil
}

func (ch *sigmaCh) SendReceive(iov sessp.IoVec) (sessp.IoVec, error) {
	idx := int(atomic.AddInt32(&ch.idx, 1))
	b, err := ch.fsls[idx%len(ch.fsls)].WriteRead(ch.fds[idx%len(ch.fds)], iov)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (ch *sigmaCh) StatsSrv() (*rpc.SigmaRPCStats, error) {
	stats := &rpc.SigmaRPCStats{}
	if err := ch.fsls[0].GetFileJson(path.Join(ch.pn, rpc.RPC, rpc.STATS), stats); err != nil {
		db.DPrintf(db.ERROR, "Error getting stats")
		return nil, err
	}
	return stats, nil
}

func NewRPCClnt(fsls []*fslib.FsLib, pn string) (*RPCClnt, error) {
	ch, err := newSigmaCh(fsls, pn)
	if err != nil {
		return nil, err
	}
	return NewRPCClntCh(ch), nil
}

// return the Blob message, if this message contains one
func getBlob(msg proto.Message) *rpcproto.Blob {
	var blob *rpcproto.Blob
	msg.ProtoReflect().Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if f.Kind() == protoreflect.MessageKind {
			if m := f.Message(); m.FullName() == "Blob" {
				blob = v.Message().Interface().(*rpcproto.Blob)
				return false
			}
		}
		return true
	})
	return blob
}

func (rpcc *RPCClnt) rpc(method string, blob sessp.IoVec) (*rpcproto.Reply, sessp.IoVec, error) {
	req := rpcproto.Request{Method: method}
	b, err := proto.Marshal(&req)
	if err != nil {
		return nil, nil, serr.NewErrError(err)
	}

	start := time.Now()

	iov, err := rpcc.ch.SendReceive(append(sessp.IoVec{b}, blob...))
	if err != nil {
		return nil, nil, err
	}

	// Record stats
	rpcc.si.Stat(method, time.Since(start).Microseconds())

	rep := &rpcproto.Reply{}
	if err := proto.Unmarshal(iov[0], rep); err != nil {
		return nil, nil, serr.NewErrError(err)
	}

	return rep, iov[1:], nil
}

// RPC handles arg and res that contain a Blob specially: it removes
// the blob from the message and pass it down in an IoVec to avoid
// marshaling overhead of large blobs.
func (rpcc *RPCClnt) RPC(method string, arg proto.Message, res proto.Message) error {
	blob := getBlob(arg)
	var iov sessp.IoVec
	if blob != nil {
		iov = blob.GetIoVec()
	}
	a, err := proto.Marshal(arg)
	if err != nil {
		return err
	}
	rep, iov, err := rpcc.rpc(method, append(sessp.IoVec{a}, iov...))
	if err != nil {
		return err
	}
	if rep.Err.ErrCode != 0 {
		return sp.NewErr(rep.Err)
	}
	if err := proto.Unmarshal(iov[0], res); err != nil {
		return err
	}
	if len(iov) > 1 { // handle blob
		blob.SetIoVec(iov[1:])
	}
	return nil
}

func (rpcc *RPCClnt) StatsClnt() map[string]*rpc.MethodStat {
	return rpcc.si.Stats()
}

func (rpcc *RPCClnt) StatsSrv() (*rpc.SigmaRPCStats, error) {
	return rpcc.ch.StatsSrv()
}
