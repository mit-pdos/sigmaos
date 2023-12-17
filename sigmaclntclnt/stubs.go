package sigmaclntclnt

import (
	"google.golang.org/protobuf/proto"

	"sigmaos/path"
	"sigmaos/serr"
	scproto "sigmaos/sigmaclntsrv/proto"
	"sigmaos/sigmaos"
	sp "sigmaos/sigmap"
)

func (scc *SigmaClntClnt) rpcErr(method string, req proto.Message, rep *scproto.SigmaErrReply) error {
	err := scc.rpcc.RPC(method, req, rep)
	if err != nil {
		return err
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
		return -1, err
	}
	if rep.Err.TErrCode() == serr.TErrNoError {
		return int(rep.Fd), nil
	} else {
		return -1, sp.NewErr(rep.Err)
	}
}

func (scc *SigmaClntClnt) rpcData(method string, req proto.Message, rep *scproto.SigmaDataReply) ([]byte, error) {
	err := scc.rpcc.RPC(method, req, rep)
	if err != nil {
		return nil, err
	}
	if rep.Err.TErrCode() == serr.TErrNoError {
		return rep.Data, nil
	} else {
		return nil, sp.NewErr(rep.Err)
	}
}

func (scc *SigmaClntClnt) rpcSize(method string, req proto.Message, rep *scproto.SigmaSizeReply) (sp.Tsize, error) {
	err := scc.rpcc.RPC(method, req, rep)
	if err != nil {
		return 0, err
	}
	if rep.Err.TErrCode() == serr.TErrNoError {
		return sp.Tsize(rep.Size), nil
	} else {
		return 0, sp.NewErr(rep.Err)
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

func (scc *SigmaClntClnt) Rename(src, dst string) error {
	req := scproto.SigmaRenameRequest{Src: src, Dst: dst}
	rep := scproto.SigmaErrReply{}
	return scc.rpcErr("SigmaClntSrv.Rename", &req, &rep)
}

func (scc *SigmaClntClnt) Remove(path string) error {
	req := scproto.SigmaRemoveRequest{Path: path}
	rep := scproto.SigmaErrReply{}
	return scc.rpcErr("SigmaClntSrv.Remove", &req, &rep)
}

func (scc *SigmaClntClnt) GetFile(path string) ([]byte, error) {
	req := scproto.SigmaGetFileRequest{Path: path}
	rep := scproto.SigmaDataReply{}
	return scc.rpcData("SigmaClntSrv.GetFile", &req, &rep)
}

func (scc *SigmaClntClnt) PutFile(path string, p sp.Tperm, m sp.Tmode, o sp.Toffset, l sp.TleaseId, data []byte) (sp.Tsize, error) {
	req := scproto.SigmaPutFileRequest{Path: path, Perm: uint32(p), Mode: uint32(m), Offset: uint64(o), LeaseId: uint64(l), Data: data}
	rep := scproto.SigmaSizeReply{}
	return scc.rpcSize("SigmaClntSrv.PutFile", &req, &rep)
}

func (scc *SigmaClntClnt) Read(fd int, sz sp.Tsize) ([]byte, error) {
	req := scproto.SigmaReadRequest{Fd: uint32(fd), Size: uint64(sz)}
	rep := scproto.SigmaDataReply{}
	return scc.rpcData("SigmaClntSrv.Read", &req, &rep)
}

func (scc *SigmaClntClnt) Write(fd int, data []byte) (sp.Tsize, error) {
	req := scproto.SigmaWriteRequest{Fd: uint32(fd), Data: data}
	rep := scproto.SigmaSizeReply{}
	return scc.rpcSize("SigmaClntSrv.Write", &req, &rep)
}

func (scc *SigmaClntClnt) Seek(fd int, off sp.Toffset) error {
	req := scproto.SigmaSeekRequest{Fd: uint32(fd), Offset: uint64(off)}
	rep := scproto.SigmaErrReply{}
	return scc.rpcErr("SigmaClntSrv.Seek", &req, &rep)
}

func (scc *SigmaClntClnt) WriteRead(fd int, data []byte) ([]byte, error) {
	req := scproto.SigmaWriteRequest{Fd: uint32(fd), Data: data}
	rep := scproto.SigmaDataReply{}
	return scc.rpcData("SigmaClntSrv.WriteRead", &req, &rep)
}

func (scc *SigmaClntClnt) CreateEphemeral(string, sp.Tperm, sp.Tmode, sp.TleaseId, sp.Tfence) (int, error) {
	return 0, nil
}

func (scc *SigmaClntClnt) ClntId() sp.TclntId {
	return 0
}

func (scc *SigmaClntClnt) FenceDir(string, sp.Tfence) error {
	return nil
}

func (scc *SigmaClntClnt) WriteFence(int, []byte, sp.Tfence) (sp.Tsize, error) {
	return 0, nil
}

func (scc *SigmaClntClnt) OpenWatch(path string, m sp.Tmode, w sigmaos.Watch) (int, error) {
	return 0, nil
}

func (scc *SigmaClntClnt) SetDirWatch(fd int, dir string, w sigmaos.Watch) error {
	return nil
}

func (scc *SigmaClntClnt) SetRemoveWatch(path string, w sigmaos.Watch) error {
	return nil
}

func (scc *SigmaClntClnt) MountTree(addrs sp.Taddrs, tree, mount string) error {
	return nil
}

func (scc *SigmaClntClnt) IsLocalMount(mnt sp.Tmount) bool {
	return false
}

func (scc *SigmaClntClnt) SetLocalMount(mnt *sp.Tmount, port string) {}

func (scc *SigmaClntClnt) PathLastMount(pn string) (path.Path, path.Path, error) {
	return path.Path{}, path.Path{}, nil
}

func (scc *SigmaClntClnt) GetNamedMount() sp.Tmount {
	return sp.NullMount()
}

func (scc *SigmaClntClnt) NewRootMount(uname string, mntname string) error {
	return nil
}

func (scc *SigmaClntClnt) Mounts() []string {
	return nil
}

func (scc *SigmaClntClnt) DetachAll() error {
	return nil
}

func (scc *SigmaClntClnt) Detach(path string) error {
	return nil
}

func (scc *SigmaClntClnt) Disconnect(path string) error {
	return nil
}
