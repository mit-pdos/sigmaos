package sigmaclntclnt

import (
	"google.golang.org/protobuf/proto"

	"sigmaos/serr"
	scproto "sigmaos/sigmaclntsrv/proto"
	sp "sigmaos/sigmap"
)

func (scc *SigmaClntClnt) rpcErr(method string, req proto.Message, rep *scproto.SigmaErrReply) error {
	err := scc.rpcc.RPC(method, req, rep)
	if err != nil {
		return nil
	}
	if rep.Err.TErrCode() == serr.TErrNoError {
		return nil
	} else {
		return sp.NewErr(rep.Err)
	}
}

func (scc *SigmaClntClnt) rpcFd(method string, req proto.Message, rep *scproto.SigmaFdReply) (int, error) {
	err := scc.rpcc.RPC(method, req, rep)
	if err != nil {
		return -1, nil
	}
	if rep.Err.TErrCode() == serr.TErrNoError {
		return int(rep.Fd), nil
	} else {
		return -1, sp.NewErr(rep.Err)
	}
}

func (scc *SigmaClntClnt) Close(fd int) error {
	req := scproto.SigmaCloseRequest{Fd: uint32(fd)}
	rep := scproto.SigmaErrReply{}
	return scc.rpcErr("SigmaClntSrv.Close", &req, &rep)
}

func (scc *SigmaClntClnt) Stat(path string) (*sp.Stat, error) {
	req := scproto.SigmaStatRequest{Path: path}
	rep := scproto.SigmaStatReply{}
	err := scc.rpcc.RPC("SigmaClntSrv.Stat", &req, &rep)
	if err != nil {
		return nil, err
	}
	if rep.Err.TErrCode() == serr.TErrNoError {
		return rep.Stat, nil
	} else {
		return nil, sp.NewErr(rep.Err)
	}
}

func (scc *SigmaClntClnt) Create(path string, p sp.Tperm, m sp.Tmode) (int, error) {
	req := scproto.SigmaCreateRequest{Path: path, Perm: uint32(p), Mode: uint32(m)}
	rep := scproto.SigmaFdReply{}
	return scc.rpcFd("SigmaClntSrv.Create", &req, &rep)
}
