package spproxyclnt

import (
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/path"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	"sigmaos/sessp"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
	spproto "sigmaos/spproxysrv/proto"
)

func (scc *SigmaClntClnt) rpcErr(method string, req proto.Message, rep *spproto.SigmaErrReply) error {
	err := scc.rpcc.RPC(method, req, rep)
	if err != nil {
		return err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return sp.NewErr(rep.Err)
	}
	return nil
}

func (scc *SigmaClntClnt) rpcFd(method string, req proto.Message, rep *spproto.SigmaFdReply) (int, error) {
	err := scc.rpcc.RPC(method, req, rep)
	if err != nil {
		return -1, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return -1, sp.NewErr(rep.Err)
	}
	return int(rep.Fd), nil
}

func (scc *SigmaClntClnt) rpcData(method string, req proto.Message, rep *spproto.SigmaDataReply) (sessp.IoVec, error) {
	err := scc.rpcc.RPC(method, req, rep)
	if err != nil {
		return nil, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil, sp.NewErr(rep.Err)
	}
	return sessp.NewIoVec(rep.Blob.Iov), nil
}

func (scc *SigmaClntClnt) rpcSize(method string, req proto.Message, rep *spproto.SigmaSizeReply) (sp.Tsize, error) {
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

func (scc *SigmaClntClnt) Init() error {
	req := spproto.SigmaInitRequest{ProcEnvProto: scc.pe.GetProto()}
	rep := spproto.SigmaErrReply{}
	err := scc.rpcErr("SPProxySrvAPI.Init", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Init %v %v %v", req, rep, err)
	return err
}

func (scc *SigmaClntClnt) CloseFd(fd int) error {
	req := spproto.SigmaCloseRequest{Fd: uint32(fd)}
	rep := spproto.SigmaErrReply{}
	err := scc.rpcErr("SPProxySrvAPI.CloseFd", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "CloseFd %v %v %v", req, rep, err)
	return err
}

func (scc *SigmaClntClnt) Stat(path string) (*sp.Stat, error) {
	req := spproto.SigmaPathRequest{Path: path}
	rep := spproto.SigmaStatReply{}
	err := scc.rpcc.RPC("SPProxySrvAPI.Stat", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Stat %v %v %v", req, rep, err)
	if err != nil {
		return nil, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil, sp.NewErr(rep.Err)
	}
	return sp.NewStatProto(rep.Stat), nil
}

func (scc *SigmaClntClnt) Create(path string, p sp.Tperm, m sp.Tmode) (int, error) {
	req := spproto.SigmaCreateRequest{Path: path, Perm: uint32(p), Mode: uint32(m)}
	rep := spproto.SigmaFdReply{}
	fd, err := scc.rpcFd("SPProxySrvAPI.Create", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Create %v %v fd %v err %v", req, rep, fd, err)
	return fd, err
}

func (scc *SigmaClntClnt) Open(path string, m sp.Tmode, w sos.Twait) (int, error) {
	req := spproto.SigmaCreateRequest{Path: path, Mode: uint32(m), Wait: bool(w)}
	rep := spproto.SigmaFdReply{}
	fd, err := scc.rpcFd("SPProxySrvAPI.Open", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Open %v %v %v %v", req, rep, fd, err)
	return fd, err
}

func (scc *SigmaClntClnt) Rename(src, dst string) error {
	req := spproto.SigmaRenameRequest{Src: src, Dst: dst}
	rep := spproto.SigmaErrReply{}
	err := scc.rpcErr("SPProxySrvAPI.Rename", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Rename %v %v %v", req, rep, err)
	return err
}

func (scc *SigmaClntClnt) Remove(path string) error {
	req := spproto.SigmaPathRequest{Path: path}
	rep := spproto.SigmaErrReply{}
	err := scc.rpcErr("SPProxySrvAPI.Remove", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Remove %v %v %v", req, rep, err)
	return err
}

func (scc *SigmaClntClnt) GetFile(path string) ([]byte, error) {
	req := spproto.SigmaPathRequest{Path: path}
	rep := spproto.SigmaDataReply{}
	rep.Blob = &rpcproto.Blob{Iov: [][]byte{nil}}
	d, err := scc.rpcData("SPProxySrvAPI.GetFile", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "GetFile %v %v %v", req, d, err)
	if err != nil {
		return nil, err
	}
	return d[0], nil
}

func (scc *SigmaClntClnt) PutFile(path string, p sp.Tperm, m sp.Tmode, data []byte, o sp.Toffset, l sp.TleaseId) (sp.Tsize, error) {
	blob := &rpcproto.Blob{Iov: [][]byte{data}}
	req := spproto.SigmaPutFileRequest{Path: path, Perm: uint32(p), Mode: uint32(m), Offset: uint64(o), LeaseId: uint64(l), Blob: blob}
	rep := spproto.SigmaSizeReply{}
	sz, err := scc.rpcSize("SPProxySrvAPI.PutFile", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "PutFile %q %v %v sz %v %v", req.Path, len(data), rep, sz, err)
	return sz, err
}

func (scc *SigmaClntClnt) Read(fd int, b []byte) (sp.Tsize, error) {
	req := spproto.SigmaReadRequest{Fd: uint32(fd), Size: uint64(len(b)), Off: uint64(sp.NoOffset)}
	rep := spproto.SigmaDataReply{}
	rep.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
	d, err := scc.rpcData("SPProxySrvAPI.Read", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Read %v size %v niov %v err %v", req, req.Size, len(d), err)
	if err != nil {
		return 0, err
	}
	return sp.Tsize(len(d[0])), nil
}

func (scc *SigmaClntClnt) Pread(fd int, b []byte, o sp.Toffset) (sp.Tsize, error) {
	req := spproto.SigmaReadRequest{Fd: uint32(fd), Size: uint64(len(b)), Off: uint64(o)}
	rep := spproto.SigmaDataReply{}
	rep.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
	d, err := scc.rpcData("SPProxySrvAPI.Read", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Read %v size %v niov %v err %v", req, req.Size, len(d), err)
	if err != nil {
		return 0, err
	}
	return sp.Tsize(len(d[0])), nil
}

func (scc *SigmaClntClnt) PreadRdr(fd int, o sp.Toffset, sz sp.Tsize) (io.ReadCloser, error) {
	db.DFatalf("PreadRdr")
	return nil, nil
}

func (scc *SigmaClntClnt) Write(fd int, data []byte) (sp.Tsize, error) {
	blob := &rpcproto.Blob{Iov: [][]byte{data}}
	req := spproto.SigmaWriteRequest{Fd: uint32(fd), Blob: blob}
	rep := spproto.SigmaSizeReply{}
	db.DPrintf(db.SIGMACLNTCLNT, "Write begin %v %v", req.Fd, len(data))
	sz, err := scc.rpcSize("SPProxySrvAPI.Write", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Write returned %v %v %v %v", req.Fd, len(data), rep, err)
	return sz, err
}

func (scc *SigmaClntClnt) Seek(fd int, off sp.Toffset) error {
	req := spproto.SigmaSeekRequest{Fd: uint32(fd), Offset: uint64(off)}
	rep := spproto.SigmaErrReply{}
	err := scc.rpcErr("SPProxySrvAPI.Seek", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Seek %v %v %v", req, rep, err)
	return err
}

func (scc *SigmaClntClnt) CreateLeased(path string, p sp.Tperm, m sp.Tmode, l sp.TleaseId, f sp.Tfence) (int, error) {
	req := spproto.SigmaCreateRequest{Path: path, Perm: uint32(p), Mode: uint32(m), LeaseId: uint64(l), Fence: f.FenceProto()}
	rep := spproto.SigmaFdReply{}
	fd, err := scc.rpcFd("SPProxySrvAPI.CreateLeased", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "CreateLeased %v %v %v", req, rep, err)
	return fd, err
}

func (scc *SigmaClntClnt) ClntId() sp.TclntId {
	req := spproto.SigmaNullRequest{}
	rep := spproto.SigmaClntIdReply{}
	err := scc.rpcc.RPC("SPProxySrvAPI.ClntId", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "ClntId %v %v", rep, err)
	if err != nil {
		return 0
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return 0
	}
	return sp.TclntId(rep.ClntId)
}

func (scc *SigmaClntClnt) FenceDir(path string, f sp.Tfence) error {
	req := spproto.SigmaFenceRequest{Path: path, Fence: f.FenceProto()}
	rep := spproto.SigmaErrReply{}
	err := scc.rpcc.RPC("SPProxySrvAPI.FenceDir", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "FenceDir %v %v %v", req, rep, err)
	return err
}

func (scc *SigmaClntClnt) WriteFence(fd int, d []byte, f sp.Tfence) (sp.Tsize, error) {
	blob := &rpcproto.Blob{Iov: [][]byte{d}}
	req := spproto.SigmaWriteRequest{Fd: uint32(fd), Blob: blob, Fence: f.FenceProto()}
	rep := spproto.SigmaSizeReply{}
	sz, err := scc.rpcSize("SPProxySrvAPI.Write", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "WriteFence %v %v %v", req, rep, err)
	return sz, err
}

func (scc *SigmaClntClnt) WriteRead(fd int, iniov sessp.IoVec, outiov sessp.IoVec) error {
	inblob := rpcproto.NewBlob(iniov)
	outblob := rpcproto.NewBlob(outiov)
	req := spproto.SigmaWriteRequest{Fd: uint32(fd), Blob: inblob, NOutVec: uint32(len(outiov))}
	rep := spproto.SigmaDataReply{Blob: outblob}
	d, err := scc.rpcData("SPProxySrvAPI.WriteRead", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "WriteRead %v %v %v %v", req.Fd, len(iniov), len(d), err)
	if err == nil && len(outiov) != len(d) {
		return fmt.Errorf("sigmaclntclnt outiov len wrong: supplied %v != %v returned", len(outiov), len(d))
	}
	copy(outiov, d)
	return err
}

func (scc *SigmaClntClnt) DirWatch(fd int) error {
	req := spproto.SigmaReadRequest{Fd: uint32(fd)}
	rep := spproto.SigmaErrReply{}
	err := scc.rpcErr("SPProxySrvAPI.DirWatch", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "DirWatch %v %v %v", req, rep, err)
	return err
}

func (scc *SigmaClntClnt) DirWatchV2(fd int) (int, error) {
	req := spproto.SigmaReadRequest{Fd: uint32(fd)}
	rep := spproto.SigmaFdReply{}
	fd, err := scc.rpcFd("SPProxySrvAPI.DirWatchV2", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "DirWatchV2 %v %v %v", req, rep, err)
	return fd, err
}

func (scc *SigmaClntClnt) MountTree(ep *sp.Tendpoint, tree, mount string) error {
	req := spproto.SigmaMountTreeRequest{Endpoint: ep.GetProto(), Tree: tree, MountName: mount}
	rep := spproto.SigmaErrReply{}
	err := scc.rpcErr("SPProxySrvAPI.MountTree", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "MountTree %v %v %v", req, rep, err)
	return err
}

func (scc *SigmaClntClnt) IsLocalMount(ep *sp.Tendpoint) (bool, error) {
	req := spproto.SigmaMountRequest{Endpoint: ep.GetProto()}
	rep := spproto.SigmaMountReply{}
	err := scc.rpcc.RPC("SPProxySrvAPI.IsLocalMount", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "IsLocalMount %v %v %v", req, rep, err)
	if err != nil {
		return false, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return false, sp.NewErr(rep.Err)
	}
	return rep.Local, nil
}

func (scc *SigmaClntClnt) PathLastMount(pn string) (path.Tpathname, path.Tpathname, error) {
	req := spproto.SigmaPathRequest{Path: pn}
	rep := spproto.SigmaLastMountReply{}
	err := scc.rpcc.RPC("SPProxySrvAPI.PathLastMount", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "PathLastMount %v %v %v", req, rep, err)
	if err != nil {
		return nil, nil, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil, nil, sp.NewErr(rep.Err)
	}
	return rep.Path1, rep.Path2, nil
}

func (scc *SigmaClntClnt) GetNamedEndpoint() (*sp.Tendpoint, error) {
	return scc.GetNamedEndpointRealm(scc.pe.GetRealm())
}

func (scc *SigmaClntClnt) InvalidateNamedEndpointCacheEntryRealm(realm sp.Trealm) error {
	req := spproto.SigmaRealmRequest{RealmStr: realm.String()}
	rep := spproto.SigmaMountReply{}
	err := scc.rpcc.RPC("SPProxySrvAPI.InvalidateNamedEndpointCacheEntryRealm", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "InvalidateNamedEndpointCacheEntryRealm %v %v %v", req, rep, err)
	if err != nil {
		return nil
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil
	}
	return nil
}

func (scc *SigmaClntClnt) GetNamedEndpointRealm(realm sp.Trealm) (*sp.Tendpoint, error) {
	req := spproto.SigmaRealmRequest{RealmStr: realm.String()}
	rep := spproto.SigmaMountReply{}
	err := scc.rpcc.RPC("SPProxySrvAPI.GetNamedEndpointRealm", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "GetNamedEndpointRealm %v %v %v", req, rep, err)
	if err != nil {
		return nil, nil
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil, nil
	}
	return sp.NewEndpointFromProto(rep.Endpoint), nil
}

func (scc *SigmaClntClnt) NewRootMount(pn, mntname string) error {
	req := spproto.SigmaMountTreeRequest{Tree: pn, MountName: mntname}
	rep := spproto.SigmaErrReply{}
	err := scc.rpcErr("SPProxySrvAPI.NewRootMount", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "NewRootMount %v %v", req, rep)
	return err
}

func (scc *SigmaClntClnt) Mounts() []string {
	req := spproto.SigmaNullRequest{}
	rep := spproto.SigmaMountsReply{}
	err := scc.rpcc.RPC("SPProxySrvAPI.Mounts", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Mounts %v %v %v", req, rep, err)
	if err != nil {
		return nil
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil
	}
	return rep.Endpoints
}

func (scc *SigmaClntClnt) SetLocalMount(ep *sp.Tendpoint, port sp.Tport) {
	db.DFatalf("SetLocalMount %v", ep)
}

func (scc *SigmaClntClnt) MountPathClnt(path string, clnt sos.PathClntAPI) error {
	return serr.NewErr(serr.TErrNotSupported, "MountPathClnt")
}

func (scc *SigmaClntClnt) Detach(path string) error {
	req := spproto.SigmaPathRequest{Path: path}
	rep := spproto.SigmaErrReply{}
	err := scc.rpcErr("SPProxySrvAPI.Detach", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Detach %v %v %v", req, rep, err)
	return err
}

func (scc *SigmaClntClnt) Disconnected() bool {
	return scc.disconnected
}

func (scc *SigmaClntClnt) Disconnect(path string) error {
	scc.disconnected = true
	req := spproto.SigmaPathRequest{Path: path}
	rep := spproto.SigmaErrReply{}
	err := scc.rpcErr("SPProxySrvAPI.Disconnect", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Disconnect %v %v %v", req, rep, err)
	return err
}

func (scc *SigmaClntClnt) Close() error {
	req := spproto.SigmaNullRequest{}
	rep := spproto.SigmaErrReply{}
	err := scc.rpcErr("SPProxySrvAPI.Close", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Close %v %v %v", req, rep, err)
	scc.close()
	return err
}
