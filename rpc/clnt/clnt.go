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
	spproxyproto "sigmaos/proxy/sigmap/proto"
	"sigmaos/rpc"
	"sigmaos/rpc/clnt/channel"
	rpcclntopts "sigmaos/rpc/clnt/opts"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

type RPCcall struct {
	method string
	iov    sessp.IoVec
}

type RPCClnt struct {
	si             *rpc.StatInfo
	ch             channel.RPCChannel
	delegatedRPCCh channel.RPCChannel
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
	delCh, err := rpcOpts.NewDelegatedRPCChannel(sp.NOT_SET)
	if err != nil {
		return nil, err
	}
	return &RPCClnt{
		si:             rpc.NewStatInfo(),
		ch:             ch,
		delegatedRPCCh: delCh,
	}, nil
}

func WrapRPCRequest(method string, arg proto.Message) (sessp.IoVec, error) {
	req := rpcproto.Req{Method: method}
	wrapperBytes, err := proto.Marshal(&req)
	if err != nil {
		return nil, serr.NewErrError(err)
	}
	var iniov sessp.IoVec
	inblob := rpc.GetBlob(arg)
	if inblob != nil {
		iniov = inblob.GetIoVec()
		inblob.SetIoVec(nil)
	}
	argBytes, err := proto.Marshal(arg)
	if err != nil {
		return nil, err
	}
	return append(sessp.IoVec{wrapperBytes, argBytes}, iniov...), nil
}

func (rpcc *RPCClnt) runWrappedRPC(delegate bool, method string, iniov sessp.IoVec, outiov sessp.IoVec) error {
	var err error
	start := time.Now()
	if delegate {
		// Sanity check
		if rpcc.delegatedRPCCh == nil {
			db.DFatalf("Try to run delegated RPC with unset delegated RPC channel")
		}
		err = rpcc.delegatedRPCCh.SendReceive(iniov, outiov)
	} else {
		// TODO: lazily init ch
		err = rpcc.ch.SendReceive(iniov, outiov)
	}
	if err != nil {
		return err
	}
	// Record stats
	rpcc.si.Stat(method, time.Since(start).Microseconds())
	return nil
}

func processWrappedRPCRep(outiov sessp.IoVec, res proto.Message, outblob *rpcproto.Blob) error {
	rep := &rpcproto.Rep{}
	if err := proto.Unmarshal(outiov[0], rep); err != nil {
		return serr.NewErrError(err)
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

// RPC handles arg and res that contain a Blob specially: it removes
// the blob from the message and pass it down in an IoVec to avoid
// marshaling overhead of large blobs.
func (rpcc *RPCClnt) RPC(method string, arg proto.Message, res proto.Message) error {
	return rpcc.rpc(false, method, arg, res)
}

func (rpcc *RPCClnt) rpc(delegate bool, method string, arg proto.Message, res proto.Message) error {
	iniov, err := WrapRPCRequest(method, arg)
	if err != nil {
		return err
	}
	// Prepend 2 empty slots to the out iovec: one for the rpcproto.Rep
	// wrapper, and one for the marshaled res proto.Message
	outiov := make(sessp.IoVec, 2)
	outblob := rpc.GetBlob(res)
	if outblob != nil { // handle blob
		// Get the reply's blob, if it has one, so that data can be read directly
		// into buffers in its IoVec
		outiov = append(outiov, outblob.GetIoVec()...)
	}
	if err := rpcc.runWrappedRPC(delegate, method, iniov, outiov); err != nil {
		return err
	}
	if err := processWrappedRPCRep(outiov, res, outblob); err != nil {
		return err
	}
	return nil
}

// DelegatedRPC handles a delegated RPC (requesting the response from
// SPProxySrv via the delegated RPC channel), retreiving a res that contains a
// Blob specially: it removes the blob from the message and pass it down in an
// IoVec to avoid marshaling overhead of large blobs.
func (rpcc *RPCClnt) DelegatedRPC(rpcIdx uint64, res proto.Message) error {
	// Prepend 2 empty slots to the out iovec: one for the rpcproto.Rep
	// wrapper, and one for the marshaled res proto.Message
	outiov := make(sessp.IoVec, 2)
	outblob := rpc.GetBlob(res)
	if outblob != nil { // handle blob
		// Get the reply's blob, if it has one, so that data can be read directly
		// into buffers in its IoVec
		outiov = append(outiov, outblob.GetIoVec()...)
	}
	req := &spproxyproto.SigmaDelegatedRPCReq{
		RPCIdx: rpcIdx,
	}
	rep := &spproxyproto.SigmaDelegatedRPCRep{
		Blob: &rpcproto.Blob{
			Iov: outiov,
		},
	}
	if err := rpcc.rpc(true, "SPProxySrvAPI.GetDelegatedRPCReply", req, rep); err != nil {
		return err
	}
	if rep.Err.ErrCode != 0 {
		return sp.NewErr(rep.Err)
	}
	return processWrappedRPCRep(rep.Blob.Iov, res, outblob)
}

func (rpcc *RPCClnt) StatsClnt() map[string]*rpc.MethodStatSnapshot {
	return rpcc.si.Stats()
}

func (rpcc *RPCClnt) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	return rpcc.ch.StatsSrv()
}

func (rpcc *RPCClnt) Channel() channel.RPCChannel {
	return rpcc.ch
}
