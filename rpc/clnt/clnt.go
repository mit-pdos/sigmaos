// Package rpcclnt is the client stub that (un)marshals argument
// protobufs and result protobufs.  By default it uses sigmaos to
// send/receive the request and receive reply.  But caller can pass
// into its own RPCCh interface to replace the default (e.g., to
// send/receive over a pipe).

package clnt

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	db "sigmaos/debug"
	spproxyproto "sigmaos/proxy/sigmap/proto"
	"sigmaos/rpc"
	"sigmaos/rpc/clnt/channel"
	"sigmaos/rpc/clnt/delegation"
	rpcclntopts "sigmaos/rpc/clnt/opts"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	"sigmaos/shmem"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

type RPCcall struct {
	method string
	iov    sessp.IoVec
}

type RPCClnt struct {
	si                    *rpc.StatInfo
	ch                    channel.RPCChannel
	delegatedRPCCh        channel.RPCChannel
	rc                    *delegation.ReplyCache
	useShmemDelegatedRPCs bool
	shmemSegment          *shmem.Segment
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
		si:                    rpc.NewStatInfo(),
		ch:                    ch,
		delegatedRPCCh:        delCh,
		rc:                    delegation.NewReplyCache(),
		useShmemDelegatedRPCs: rpcOpts.UseShmemDelegatedRPCs,
		shmemSegment:          rpcOpts.ShmemSegment,
	}, nil
}

func WrapRPCRequest(method string, arg proto.Message) (*sessp.IoVec, error) {
	iniov := sessp.NewUnallocatedIoVec(0, nil)
	inblob := rpc.GetBlob(arg)
	if inblob != nil {
		iniov = inblob.GetIoVec()
		inblob.ClearIoVec()
	}
	argBytes, err := proto.Marshal(arg)
	if err != nil {
		return nil, err
	}
	iniov.InsertFrame(0, sessp.NewFrame(argBytes, nil))
	return WrapMarshaledRPCRequest(method, iniov)
}

func WrapMarshaledRPCRequest(method string, iniov *sessp.IoVec) (*sessp.IoVec, error) {
	req := rpcproto.Req{Method: method}
	wrapperBytes, err := proto.Marshal(&req)
	if err != nil {
		return nil, serr.NewErrError(err)
	}
	iniov.InsertFrame(0, sessp.NewFrame(wrapperBytes, nil))
	return iniov, nil
}

func (rpcc *RPCClnt) runWrappedRPC(delegate bool, method string, iniov *sessp.IoVec, outiov *sessp.IoVec) error {
	var err error
	start := time.Now()
	if delegate {
		// Sanity check
		if rpcc.delegatedRPCCh == nil {
			db.DFatalf("Try to run delegated RPC with unset delegated RPC channel")
		}
		err = rpcc.delegatedRPCCh.SendReceive(iniov, outiov)
	} else {
		err = rpcc.ch.SendReceive(iniov, outiov)
	}
	if err != nil {
		return err
	}
	// Record stats
	rpcc.si.Stat(method, time.Since(start).Microseconds())
	return nil
}

func processWrappedRPCRep(outiov *sessp.IoVec, res proto.Message, outblob *rpcproto.Blob) error {
	if outiov.Len() == 0 {
		return serr.NewErr(serr.TErrUnreachable, "empty RPC reply")
	}
	rep := &rpcproto.Rep{}
	if err := proto.Unmarshal(outiov.GetFrame(0).GetBuf(), rep); err != nil {
		return serr.NewErrError(err)
	}
	if rep.Err.ErrCode != 0 {
		return sp.NewErr(rep.Err)
	}
	// A reply that carries no error must carry the result message (and its blob
	// frames, if any). Fewer frames than that means the reply was truncated;
	// report it rather than indexing past the end.
	if outiov.Len() < 2 {
		return serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("short RPC reply: %d frames, no error", outiov.Len()))
	}
	if err := proto.Unmarshal(outiov.GetFrame(1).GetBuf(), res); err != nil {
		return err
	}
	if outblob != nil {
		// Need to get the blob again, because its value will be reset during
		// unmarshaling
		outblob = rpc.GetBlob(res)
		// Set the IoVec to handle replies with blobs
		outblob.SetIoVecBufs(outiov.ToByteSlices()[2:])
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
	outiov := sessp.NewUnallocatedIoVec(2, nil)
	outblob := rpc.GetBlob(res)
	if outblob != nil { // handle blob
		// Get the reply's blob, if it has one, so that data can be read directly
		// into buffers in its IoVec
		outiov.AppendFrames(outblob.GetIoVec().GetFrames())
	}
	if err := rpcc.runWrappedRPC(delegate, method, iniov, outiov); err != nil {
		return err
	}
	if err := processWrappedRPCRep(outiov, res, outblob); err != nil {
		return err
	}
	return nil
}

// OutgoingDelegatedRPC sends an RPC to the boot script via the SPProxySrv
func (rpcc *RPCClnt) OutgoingDelegatedRPC(rpcIdx uint64, method string, arg proto.Message) error {
	iniov, err := WrapRPCRequest(method, arg)
	if err != nil {
		return err
	}
	blob := &rpcproto.Blob{}
	blob.SetIoVec(iniov)
	req := &spproxyproto.SigmaOutgoingDelegatedRPCReq{
		RPCIdx:          rpcIdx,
		Blob:            blob,
		TransferStartPB: timestamppb.New(time.Now()),
	}
	rep := &spproxyproto.SigmaErrRep{}
	if err := rpcc.rpc(true, "SPProxySrvAPI.OutgoingDelegatedRPC", req, rep); err != nil {
		db.DPrintf(db.ERROR, "Err OutgoingDelegatedRPC(%v): %v", rpcIdx, err)
		return err
	}
	if rep.Err.ErrCode != 0 {
		return sp.NewErr(rep.Err)
	}
	return nil
}

// DelegatedRPC handles a delegated RPC (requesting the response from
// SPProxySrv via the delegated RPC channel), retreiving a res that contains a
// Blob specially: it removes the blob from the message and pass it down in an
// IoVec to avoid marshaling overhead of large blobs.
func (rpcc *RPCClnt) DelegatedRPC(rpcIdx uint64, res proto.Message) (time.Duration, error) {
	var nIOVec int
	if !rpcc.useShmemDelegatedRPCs {
		// Prepend 2 empty slots to the out iovec: one for the rpcproto.Rep
		// wrapper, and one for the marshaled res proto.Message
		nIOVec = 2
	} else {
		// Prepend nothing to out IOV. Everything will be passed in shared memory
	}
	outiov := sessp.NewUnallocatedIoVec(nIOVec, nil)
	outblob := rpc.GetBlob(res)
	if !rpcc.useShmemDelegatedRPCs {
		// If not using shared memory, append blob IOVecs to the out IOVec
		if outblob != nil { // handle blob
			// Get the reply's blob, if it has one, so that data can be read directly
			// into buffers in its IoVec
			outiov.AppendFrames(outblob.GetIoVec().GetFrames())
		}
	}
	req := &spproxyproto.SigmaDelegatedRPCReq{
		RPCIdx:   rpcIdx,
		UseShmem: rpcc.useShmemDelegatedRPCs,
	}
	rep := &spproxyproto.SigmaDelegatedRPCRep{
		Blob: &rpcproto.Blob{
			Iov: outiov.ToByteSlices(),
		},
	}
	start := time.Now()
	// Check if the reply has already been cached client-side
	err, ok := rpcc.rc.Get(rpcIdx, rep)
	if !ok {
		rpcc.rc.Register(rpcIdx)
		// If delegated RPC reply wasn't cached on the client-side, request it from
		// the SPProxy
		err = rpcc.rpc(true, "SPProxySrvAPI.GetDelegatedRPCReply", req, rep)
		rpcc.rc.Put(rpcIdx, rep, err)
	} else {
		// If delegated RPC reply was cached on the client-side
		db.DPrintf(db.RPCCLNT, "Get DelegatedRPC(%v) cached", rpcIdx)
	}
	if err != nil {
		return 0, err
	}
	// Before touching the reply's frames: the delegated RPC itself may have
	// failed, in which case there is no result message or blob to map — only the
	// error. Checking after the frame handling below meant an errored delegated
	// RPC underflowed the frame count to -1 and died on the sanity check with a
	// message about buffer counts, hiding the error that actually happened.
	if rep.Err.ErrCode != 0 {
		return 0, sp.NewErr(rep.Err)
	}
	if rpcc.useShmemDelegatedRPCs {
		// Set IOVec from shared memory region
		if rep.UseShmem {
			// The reply is the wrapper, the result message, and one frame per
			// blob buffer the caller supplied. Fewer frames than that (with no
			// error reported above) means a truncated reply: report it rather
			// than mapping frames that aren't there.
			if len(rep.ShmOffs) < 2 {
				return 0, serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("short shmem delegated RPC(%v) reply: %d frames", rpcIdx, len(rep.ShmOffs)))
			}
			if len(outblob.Iov) != len(rep.ShmOffs)-2 {
				db.DFatalf("Wrong number of buffers supplied for shared-memory delegated RPC: %v != %v", len(outblob.Iov), len(rep.ShmOffs)-2)
			}
			frames := make([]*sessp.Tframe, len(rep.ShmOffs))
			b := rpcc.shmemSegment.GetBuf()
			for i := range rep.ShmOffs {
				start := rep.ShmOffs[i]
				end := start + rep.ShmLens[i]
				// The offsets come from another process; a frame that doesn't lie
				// inside this segment would otherwise panic on the slice, or
				// silently hand the caller unrelated bytes to unmarshal.
				if end < start || end > uint64(len(b)) {
					return 0, serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("shmem delegated RPC(%v) frame %d of %d out of segment: [%v,%v) size %v", rpcIdx, i, len(rep.ShmOffs), start, end, len(b)))
				}
				frames[i] = sessp.NewFrame(b[start:end], nil)
			}
			outiov.AppendFrames(frames)
			rep.Blob.SetIoVec(outiov)
		} else {
			db.DFatalf("Err: rpcclnt using shared memory but reply isn't using shared memory")
		}
	}
	transferDur := time.Since(rep.TransferStartPB.AsTime())
	perf.LogSpawnLatency("DelegatedRPC.RunRPC %d", sp.NOT_SET, perf.TIME_NOT_SET, start, rpcIdx)
	start = time.Now()
	defer func(start time.Time) {
		perf.LogSpawnLatency("DelegatedRPC.Unmarshal %d", sp.NOT_SET, perf.TIME_NOT_SET, start, rpcIdx)
	}(start)
	return transferDur, processWrappedRPCRep(rep.Blob.GetIoVec(), res, outblob)
}

// Fetch a batch of delegated RPC results
func (rpcc *RPCClnt) BatchFetchDelegatedRPCs(idxs []uint64, nIOV int) error {
	req := &spproxyproto.SigmaMultiDelegatedRPCReq{
		RPCIdxs: idxs,
	}
	multiRep := &spproxyproto.SigmaMultiDelegatedRPCRep{
		Blob: &rpcproto.Blob{
			Iov: make([][]byte, nIOV),
		},
	}
	err := rpcc.rpc(true, "SPProxySrvAPI.GetMultiDelegatedRPCReplies", req, multiRep)
	if err != nil {
		return err
	}
	start := 0
	for i, rpcIdx := range idxs {
		rpcc.rc.Register(rpcIdx)
		end := start + int(multiRep.NIOVs[i])
		rep := &spproxyproto.SigmaDelegatedRPCRep{
			Blob: &rpcproto.Blob{
				Iov: multiRep.Blob.Iov[start:end],
			},
			Err: multiRep.Errs[i],
		}
		rpcc.rc.Put(rpcIdx, rep, err)
		start = end
	}
	return nil
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
