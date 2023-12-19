package sigmaclntclnt

import (
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
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
	if rep.Err.TErrCode() != serr.TErrNoError {
		return sp.NewErr(rep.Err)
	}
	return nil
}

func (scc *SigmaClntClnt) rpcFd(method string, req proto.Message, rep *scproto.SigmaFdReply) (int, error) {
	err := scc.rpcc.RPC(method, req, rep)
	if err != nil {
		return -1, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return -1, sp.NewErr(rep.Err)
	}
	return int(rep.Fd), nil
}

func (scc *SigmaClntClnt) rpcData(method string, req proto.Message, rep *scproto.SigmaDataReply) ([]byte, error) {
	err := scc.rpcc.RPC(method, req, rep)
	db.DPrintf(db.SIGMACLNTCLNT, "rpcData: %v %d %v\n", err, len(rep.Data), rep.Err)
	if err != nil {
		return nil, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil, sp.NewErr(rep.Err)
	}
	return rep.Data, nil
}

func (scc *SigmaClntClnt) rpcSize(method string, req proto.Message, rep *scproto.SigmaSizeReply) (sp.Tsize, error) {
	err := scc.rpcc.RPC(method, req, rep)
	db.DPrintf(db.SIGMACLNTCLNT, "rpcSize: %v %v %v\n", err, rep.Size, rep.Err)
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
	db.DPrintf(db.SIGMACLNTCLNT, "Close %v", req)
	return scc.rpcErr("SigmaClntSrv.Close", &req, &rep)
}

func (scc *SigmaClntClnt) Stat(path string) (*sp.Stat, error) {
	req := scproto.SigmaPathRequest{Path: path}
	rep := scproto.SigmaStatReply{}
	db.DPrintf(db.SIGMACLNTCLNT, "Stat %v", req)
	err := scc.rpcc.RPC("SigmaClntSrv.Stat", &req, &rep)
	if err != nil {
		return nil, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil, sp.NewErr(rep.Err)
	}
	return rep.Stat, nil
}

func (scc *SigmaClntClnt) Create(path string, p sp.Tperm, m sp.Tmode) (int, error) {
	req := scproto.SigmaCreateRequest{Path: path, Perm: uint32(p), Mode: uint32(m)}
	rep := scproto.SigmaFdReply{}
	fd, err := scc.rpcFd("SigmaClntSrv.Create", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Create %v %v", req, rep)
	return fd, err
}

func (scc *SigmaClntClnt) Open(path string, m sp.Tmode) (int, error) {
	req := scproto.SigmaCreateRequest{Path: path, Mode: uint32(m)}
	rep := scproto.SigmaFdReply{}
	fd, err := scc.rpcFd("SigmaClntSrv.Open", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Open %v %v", req, rep)
	return fd, err
}

func (scc *SigmaClntClnt) Rename(src, dst string) error {
	req := scproto.SigmaRenameRequest{Src: src, Dst: dst}
	rep := scproto.SigmaErrReply{}
	err := scc.rpcErr("SigmaClntSrv.Rename", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Rename %v %v", req, rep)
	return err
}

func (scc *SigmaClntClnt) Remove(path string) error {
	req := scproto.SigmaPathRequest{Path: path}
	rep := scproto.SigmaErrReply{}
	db.DPrintf(db.SIGMACLNTCLNT, "Remove %v", req)
	return scc.rpcErr("SigmaClntSrv.Remove", &req, &rep)
}

func (scc *SigmaClntClnt) GetFile(path string) ([]byte, error) {
	req := scproto.SigmaPathRequest{Path: path}
	rep := scproto.SigmaDataReply{}
	b, err := scc.rpcData("SigmaClntSrv.GetFile", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "GetFile %v", req, rep)
	return b, err
}

func (scc *SigmaClntClnt) PutFile(path string, p sp.Tperm, m sp.Tmode, data []byte, o sp.Toffset, l sp.TleaseId) (sp.Tsize, error) {
	req := scproto.SigmaPutFileRequest{Path: path, Perm: uint32(p), Mode: uint32(m), Offset: uint64(o), LeaseId: uint64(l), Data: data}
	rep := scproto.SigmaSizeReply{}
	sz, err := scc.rpcSize("SigmaClntSrv.PutFile", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "PutFile %v", req, rep)
	return sz, err
}

func (scc *SigmaClntClnt) Read(fd int, sz sp.Tsize) ([]byte, error) {
	req := scproto.SigmaReadRequest{Fd: uint32(fd), Size: uint64(sz)}
	rep := scproto.SigmaDataReply{}
	b, err := scc.rpcData("SigmaClntSrv.Read", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Read %v %v", req, rep)
	return b, err
}

func (scc *SigmaClntClnt) Write(fd int, data []byte) (sp.Tsize, error) {
	req := scproto.SigmaWriteRequest{Fd: uint32(fd), Data: data}
	rep := scproto.SigmaSizeReply{}
	sz, err := scc.rpcSize("SigmaClntSrv.Write", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Write %v %v", req, rep)
	return sz, err
}

func (scc *SigmaClntClnt) Seek(fd int, off sp.Toffset) error {
	req := scproto.SigmaSeekRequest{Fd: uint32(fd), Offset: uint64(off)}
	rep := scproto.SigmaErrReply{}
	err := scc.rpcErr("SigmaClntSrv.Seek", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Seek %v", req, rep)
	return err
}

func (scc *SigmaClntClnt) WriteRead(fd int, data []byte) ([]byte, error) {
	req := scproto.SigmaWriteRequest{Fd: uint32(fd), Data: data}
	rep := scproto.SigmaDataReply{}
	b, err := scc.rpcData("SigmaClntSrv.WriteRead", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "WriteRead %v %v", req, rep)
	return b, err
}

func (scc *SigmaClntClnt) CreateEphemeral(path string, p sp.Tperm, m sp.Tmode, l sp.TleaseId, f sp.Tfence) (int, error) {
	db.DPrintf(db.SIGMACLNTCLNT, "CreateEphemeral %v", path)
	return 0, nil
}

func (scc *SigmaClntClnt) ClntId() sp.TclntId {
	db.DPrintf(db.SIGMACLNTCLNT, "ClntID")
	return 0
}

func (scc *SigmaClntClnt) FenceDir(path string, f sp.Tfence) error {
	db.DPrintf(db.SIGMACLNTCLNT, "FenceDir %v", path)
	return nil
}

func (scc *SigmaClntClnt) WriteFence(fd int, d []byte, f sp.Tfence) (sp.Tsize, error) {
	db.DPrintf(db.SIGMACLNTCLNT, "WriteFence %v", fd)
	return 0, nil
}

func (scc *SigmaClntClnt) OpenWatch(path string, m sp.Tmode, w sigmaos.Watch) (int, error) {
	db.DPrintf(db.SIGMACLNTCLNT, "OpenWatch %v", path)
	return 0, nil
}

func (scc *SigmaClntClnt) SetDirWatch(fd int, dir string, w sigmaos.Watch) error {
	db.DPrintf(db.SIGMACLNTCLNT, "SetDirWatch %v", dir)
	return nil
}

func (scc *SigmaClntClnt) SetRemoveWatch(path string, w sigmaos.Watch) error {
	db.DPrintf(db.SIGMACLNTCLNT, "SetRemoveWatch %v", path)
	return nil
}

func (scc *SigmaClntClnt) MountTree(addrs sp.Taddrs, tree, mount string) error {
	req := scproto.SigmaMountTreeRequest{Addr: addrs, Tree: tree, Mount: mount}
	rep := scproto.SigmaErrReply{}
	err := scc.rpcErr("SigmaClntSrv.MountTree", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "MountTree %v %v", req, rep)
	return err
}

func (scc *SigmaClntClnt) IsLocalMount(mnt sp.Tmount) bool {
	db.DPrintf(db.SIGMACLNTCLNT, "IsLocalMount %v", mnt)
	return true
}

func (scc *SigmaClntClnt) SetLocalMount(mnt *sp.Tmount, port string) {
	db.DPrintf(db.SIGMACLNTCLNT, "SetLocalMount %v", mnt)
}

func (scc *SigmaClntClnt) PathLastMount(pn string) (path.Path, path.Path, error) {
	req := scproto.SigmaPathRequest{Path: pn}
	rep := scproto.SigmaLastMountReply{}
	err := scc.rpcc.RPC("SigmaClntSrv.PathLastMount", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "PathLastMount %v", req, rep)
	if err != nil {
		return nil, nil, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil, nil, sp.NewErr(rep.Err)
	}
	return rep.Path1, rep.Path2, nil
}

func (scc *SigmaClntClnt) GetNamedMount() sp.Tmount {
	db.DPrintf(db.SIGMACLNTCLNT, "GetNamedMount")
	req := scproto.SigmaNullRequest{}
	rep := scproto.SigmaMountReply{}
	err := scc.rpcc.RPC("SigmaClntSrv.GetNamedMount", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "PathLastMount %v", req, rep)
	if err != nil {
		return sp.NullMount()
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return sp.NullMount()
	}
	return sp.Tmount{rep.Mount}
}

func (scc *SigmaClntClnt) NewRootMount(uname string, mntname string) error {
	db.DPrintf(db.SIGMACLNTCLNT, "NewRootMount %v", uname)
	return nil
}

func (scc *SigmaClntClnt) Mounts() []string {
	db.DPrintf(db.SIGMACLNTCLNT, "Mounts")
	return nil
}

func (scc *SigmaClntClnt) DetachAll() error {
	db.DPrintf(db.SIGMACLNTCLNT, "Detachall")
	return nil
}

func (scc *SigmaClntClnt) Detach(path string) error {
	db.DPrintf(db.SIGMACLNTCLNT, "Detach %v", path)
	return nil
}

func (scc *SigmaClntClnt) Disconnect(path string) error {
	req := scproto.SigmaPathRequest{Path: path}
	rep := scproto.SigmaErrReply{}
	err := scc.rpcErr("SigmaClntSrv.Disconnect", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Disconnect %v %v", req, rep)
	return err
}
