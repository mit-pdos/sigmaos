// Package rpcclnt is the client stub that (un)marshals argument
// protobufs and result protobufs.  By default it uses sigmaos to
// send/receive the request and receive reply.  But caller can pass
// into its own RPCCh interface to replace the default (e.g., to
// send/receive over a pipe).

package clnt

import (
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/rpc"
	"sigmaos/rpc/clnt/channel"
	rpcclntopts "sigmaos/rpc/clnt/opts"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type RPCcall struct {
	method string
	iov    sessp.IoVec
}

type RPCClnt struct {
	si *rpc.StatInfo
	ch channel.RPCChannel
}

// XXX TODO Shouldn't take pn here
func NewRPCClnt(pn string, opts ...*rpcclntopts.RPCClntOption) (*RPCClnt, error) {
	s := time.Now()
	defer func() {
		db.DPrintf(db.ATTACH_LAT, "NewSigmaRPCClnt %q lat %v", pn, time.Since(s))
	}()

	rpcOpts := rpcclntopts.NewEmptyRPCClntOptions()
	for _, opt := range opts {
		opt.Apply(rpcOpts)
	}

	ch, err := rpcOpts.NewRPCChannel(pn)
	if err != nil {
		return nil, err
	}
	return &RPCClnt{
		si: rpc.NewStatInfo(),
		ch: ch,
	}, nil
}

func (rpcc *RPCClnt) rpc(method string, iniov sessp.IoVec, outiov sessp.IoVec) (*rpcproto.Reply, error) {
	req := rpcproto.Request{Method: method}
	b, err := proto.Marshal(&req)
	if err != nil {
		return nil, serr.NewErrError(err)
	}

	start := time.Now()
	err = rpcc.ch.SendReceive(append(sessp.IoVec{b}, iniov...), outiov)
	if err != nil {
		return nil, err
	}
	// Record stats
	rpcc.si.Stat(method, time.Since(start).Microseconds())

	rep := &rpcproto.Reply{}
	if err := proto.Unmarshal(outiov[0], rep); err != nil {
		return nil, serr.NewErrError(err)
	}
	return rep, nil
}

// RPC handles arg and res that contain a Blob specially: it removes
// the blob from the message and pass it down in an IoVec to avoid
// marshaling overhead of large blobs.
func (rpcc *RPCClnt) RPC(method string, arg proto.Message, res proto.Message) error {
	inblob := rpc.GetBlob(arg)
	var iniov sessp.IoVec
	if inblob != nil {
		iniov = inblob.GetIoVec()
		inblob.SetIoVec(nil)
	}
	a, err := proto.Marshal(arg)
	if err != nil {
		return err
	}
	// Prepend 2 empty slots to the out iovec: one for the rpcproto.Reply
	// wrapper, and one for the marshaled res proto.Message
	outiov := make(sessp.IoVec, 2)
	outblob := rpc.GetBlob(res)
	if outblob != nil { // handle blob
		// Get the reply's blob, if it has one, so that data can be read directly
		// into buffers in its IoVec
		outiov = append(outiov, outblob.GetIoVec()...)
	}
	// Add an IoVec spot for the RPC wrappers
	rep, err := rpcc.rpc(method, append(sessp.IoVec{a}, iniov...), outiov)
	if err != nil {
		return err
	}
	if rep.Err.ErrCode != 0 {
		return sp.NewErr(rep.Err)
	}
	if err := proto.Unmarshal(outiov[1], res); err != nil {
		return err
	}
	if outblob != nil {
		// Need to get the blob again, because its value will be reset during
		// unmarshaling
		outblob = rpc.GetBlob(res)
		// Set the IoVec to handle replies with blobs
		outblob.SetIoVec(outiov[2:])
	}
	return nil
}

func (rpcc *RPCClnt) StatsClnt() map[string]*rpc.MethodStatSnapshot {
	return rpcc.si.Stats()
}

func (rpcc *RPCClnt) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	return rpcc.ch.StatsSrv()
}
