package rpcclnt

import (
	"path"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/rpc"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	"sigmaos/sessdevclnt"
	sp "sigmaos/sigmap"
)

type RPCCh interface {
	WriteRead([]byte) ([]byte, error)
	StatsSrv() (*rpc.SigmaRPCStats, error)
}

type RPCClnt struct {
	si *rpc.StatInfo
	ch RPCCh
}

func NewRPCClntCh(ch RPCCh) (*RPCClnt, error) {
	rpcc := &RPCClnt{
		si: rpc.NewStatInfo(),
		ch: ch,
	}
	return rpcc, nil
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

func (ch *sigmaCh) WriteRead(b []byte) ([]byte, error) {
	idx := int(atomic.AddInt32(&ch.idx, 1))
	b, err := ch.fsls[idx%len(ch.fsls)].WriteRead(ch.fds[idx%len(ch.fds)], b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (ch *sigmaCh) StatsSrv() (*rpc.SigmaRPCStats, error) {
	stats := &rpc.SigmaRPCStats{}
	if err := ch.fsls[0].GetFileJson(path.Join(ch.pn, rpc.RPC, rpc.STATS), stats); err != nil {
		db.DFatalf("Error getting stats")
		return nil, err
	}
	return stats, nil
}

func NewRPCClnt(fsls []*fslib.FsLib, pn string) (*RPCClnt, error) {
	ch, err := newSigmaCh(fsls, pn)
	if err != nil {
		return nil, err
	}
	return NewRPCClntCh(ch)
}

func (rpcc *RPCClnt) rpc(method string, a []byte) (*rpcproto.Reply, error) {
	req := rpcproto.Request{Method: method, Args: a}
	b, err := proto.Marshal(&req)
	if err != nil {
		return nil, serr.NewErrError(err)
	}

	start := time.Now()

	b1, err := rpcc.ch.WriteRead(b)
	if err != nil {
		return nil, serr.NewErrError(err)
	}

	// Record stats
	rpcc.si.Stat(method, time.Since(start).Microseconds())

	rep := &rpcproto.Reply{}
	if err := proto.Unmarshal(b1, rep); err != nil {
		return nil, serr.NewErrError(err)
	}

	return rep, nil
}

func (rpcc *RPCClnt) RPC(method string, arg proto.Message, res proto.Message) error {
	b, err := proto.Marshal(arg)
	if err != nil {
		return err
	}
	rep, err := rpcc.rpc(method, b)
	if err != nil {
		return err
	}
	if rep.Err.ErrCode != 0 {
		return sp.NewErr(rep.Err)
	}
	if err := proto.Unmarshal(rep.Res, res); err != nil {
		return err
	}
	return nil
}

func (rpcc *RPCClnt) StatsClnt() map[string]*rpc.MethodStat {
	return rpcc.si.Stats()
}

func (rpcc *RPCClnt) StatsSrv() (*rpc.SigmaRPCStats, error) {
	return rpcc.ch.StatsSrv()
}
