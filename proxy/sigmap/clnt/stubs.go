package clnt

import (
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"

	sos "sigmaos/api/sigmaos"
	db "sigmaos/debug"
	"sigmaos/path"
	spproto "sigmaos/proxy/sigmap/proto"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

func (scc *SPProxyClnt) rpcErr(method string, req proto.Message, rep *spproto.SigmaErrRep) error {
	err := scc.rpcc.RPC(method, req, rep)
	if err != nil {
		return err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return sp.NewErr(rep.Err)
	}
	return nil
}

func (scc *SPProxyClnt) rpcFd(method string, req proto.Message, rep *spproto.SigmaFdRep) (int, error) {
	err := scc.rpcc.RPC(method, req, rep)
	if err != nil {
		return -1, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return -1, sp.NewErr(rep.Err)
	}
	return int(rep.Fd), nil
}

func (scc *SPProxyClnt) rpcData(method string, req proto.Message, rep *spproto.SigmaDataRep) (sessp.IoVec, error) {
	err := scc.rpcc.RPC(method, req, rep)
	if err != nil {
		return nil, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil, sp.NewErr(rep.Err)
	}
	return sessp.NewIoVec(rep.Blob.Iov), nil
}

func (scc *SPProxyClnt) rpcSize(method string, req proto.Message, rep *spproto.SigmaSizeRep) (sp.Tsize, error) {
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

func (scc *SPProxyClnt) Init() error {
	req := spproto.SigmaInitReq{ProcEnvProto: scc.pe.GetProto()}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.Init", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Init %v %v %v", req, rep, err)
	return err
}

func (scc *SPProxyClnt) CloseFd(fd int) error {
	req := spproto.SigmaCloseReq{Fd: uint32(fd)}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.CloseFd", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "CloseFd %v %v %v", req, rep, err)
	return err
}

func (scc *SPProxyClnt) Stat(path string) (*sp.Tstat, error) {
	req := spproto.SigmaPathReq{Path: path}
	rep := spproto.SigmaStatRep{}
	err := scc.rpcc.RPC("SPProxySrvAPI.Stat", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Stat %v %v %v", req, rep, err)
	if err != nil {
		return nil, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil, sp.NewErr(rep.Err)
	}
	return sp.NewStatProto(rep.Stat), nil
}

func (scc *SPProxyClnt) Create(path string, p sp.Tperm, m sp.Tmode) (int, error) {
	req := spproto.SigmaCreateReq{Path: path, Perm: uint32(p), Mode: uint32(m)}
	rep := spproto.SigmaFdRep{}
	fd, err := scc.rpcFd("SPProxySrvAPI.Create", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Create %v %v fd %v err %v", req, rep, fd, err)
	return fd, err
}

func (scc *SPProxyClnt) Open(path string, m sp.Tmode, w sos.Twait) (int, error) {
	req := spproto.SigmaCreateReq{Path: path, Mode: uint32(m), Wait: bool(w)}
	rep := spproto.SigmaFdRep{}
	fd, err := scc.rpcFd("SPProxySrvAPI.Open", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Open %v %v %v %v", req, rep, fd, err)
	return fd, err
}

func (scc *SPProxyClnt) Rename(src, dst string) error {
	req := spproto.SigmaRenameReq{Src: src, Dst: dst}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.Rename", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Rename %v %v %v", req, rep, err)
	return err
}

func (scc *SPProxyClnt) Remove(path string) error {
	req := spproto.SigmaPathReq{Path: path}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.Remove", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Remove %v %v %v", req, rep, err)
	return err
}

func (scc *SPProxyClnt) GetFile(path string) ([]byte, error) {
	req := spproto.SigmaPathReq{Path: path}
	rep := spproto.SigmaDataRep{}
	rep.Blob = &rpcproto.Blob{Iov: [][]byte{nil}}
	d, err := scc.rpcData("SPProxySrvAPI.GetFile", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "GetFile %v %v %v", req, d, err)
	if err != nil {
		return nil, err
	}
	return d[0], nil
}

func (scc *SPProxyClnt) PutFile(path string, p sp.Tperm, m sp.Tmode, data []byte, o sp.Toffset, l sp.TleaseId) (sp.Tsize, error) {
	blob := &rpcproto.Blob{Iov: [][]byte{data}}
	req := spproto.SigmaPutFileReq{Path: path, Perm: uint32(p), Mode: uint32(m), Offset: uint64(o), LeaseId: uint64(l), Blob: blob}
	rep := spproto.SigmaSizeRep{}
	sz, err := scc.rpcSize("SPProxySrvAPI.PutFile", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "PutFile %q %v %v sz %v %v", req.Path, len(data), rep, sz, err)
	return sz, err
}

func (scc *SPProxyClnt) Read(fd int, b []byte) (sp.Tsize, error) {
	req := spproto.SigmaReadReq{Fd: uint32(fd), Size: uint64(len(b)), Off: uint64(sp.NoOffset)}
	rep := spproto.SigmaDataRep{}
	rep.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
	d, err := scc.rpcData("SPProxySrvAPI.Read", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Read %v size %v niov %v err %v", req, req.Size, len(d), err)
	if err != nil {
		return 0, err
	}
	return sp.Tsize(len(d[0])), nil
}

func (scc *SPProxyClnt) Pread(fd int, b []byte, o sp.Toffset) (sp.Tsize, error) {
	req := spproto.SigmaReadReq{Fd: uint32(fd), Size: uint64(len(b)), Off: uint64(o)}
	rep := spproto.SigmaDataRep{}
	rep.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
	d, err := scc.rpcData("SPProxySrvAPI.Read", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Read %v size %v niov %v err %v", req, req.Size, len(d), err)
	if err != nil {
		return 0, err
	}
	return sp.Tsize(len(d[0])), nil
}

func (scc *SPProxyClnt) PreadRdr(fd int, o sp.Toffset, sz sp.Tsize) (io.ReadCloser, error) {
	db.DFatalf("PreadRdr")
	return nil, nil
}

func (scc *SPProxyClnt) Write(fd int, data []byte) (sp.Tsize, error) {
	blob := &rpcproto.Blob{Iov: [][]byte{data}}
	req := spproto.SigmaWriteReq{Fd: uint32(fd), Blob: blob}
	rep := spproto.SigmaSizeRep{}
	db.DPrintf(db.SPPROXYCLNT, "Write begin %v %v", req.Fd, len(data))
	sz, err := scc.rpcSize("SPProxySrvAPI.Write", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Write returned %v %v %v %v", req.Fd, len(data), rep, err)
	return sz, err
}

func (scc *SPProxyClnt) Seek(fd int, off sp.Toffset) error {
	req := spproto.SigmaSeekReq{Fd: uint32(fd), Offset: uint64(off)}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.Seek", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Seek %v %v %v", req, rep, err)
	return err
}

func (scc *SPProxyClnt) CreateLeased(path string, p sp.Tperm, m sp.Tmode, l sp.TleaseId, f sp.Tfence) (int, error) {
	req := spproto.SigmaCreateReq{Path: path, Perm: uint32(p), Mode: uint32(m), LeaseId: uint64(l), Fence: f.FenceProto()}
	rep := spproto.SigmaFdRep{}
	fd, err := scc.rpcFd("SPProxySrvAPI.CreateLeased", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "CreateLeased %v %v %v", req, rep, err)
	return fd, err
}

func (scc *SPProxyClnt) ClntId() sp.TclntId {
	req := spproto.SigmaNullReq{}
	rep := spproto.SigmaClntIdRep{}
	err := scc.rpcc.RPC("SPProxySrvAPI.ClntId", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "ClntId %v %v", rep, err)
	if err != nil {
		return 0
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return 0
	}
	return sp.TclntId(rep.ClntId)
}

func (scc *SPProxyClnt) FenceDir(path string, f sp.Tfence) error {
	req := spproto.SigmaFenceReq{Path: path, Fence: f.FenceProto()}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcc.RPC("SPProxySrvAPI.FenceDir", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "FenceDir %v %v %v", req, rep, err)
	return err
}

func (scc *SPProxyClnt) WriteFence(fd int, d []byte, f sp.Tfence) (sp.Tsize, error) {
	blob := &rpcproto.Blob{Iov: [][]byte{d}}
	req := spproto.SigmaWriteReq{Fd: uint32(fd), Blob: blob, Fence: f.FenceProto()}
	rep := spproto.SigmaSizeRep{}
	sz, err := scc.rpcSize("SPProxySrvAPI.Write", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "WriteFence %v %v %v", req, rep, err)
	return sz, err
}

func (scc *SPProxyClnt) WriteRead(fd int, iniov sessp.IoVec, outiov sessp.IoVec) error {
	inblob := rpcproto.NewBlob(iniov)
	outblob := rpcproto.NewBlob(outiov)
	req := spproto.SigmaWriteReq{Fd: uint32(fd), Blob: inblob, NOutVec: uint32(len(outiov))}
	rep := spproto.SigmaDataRep{Blob: outblob}
	d, err := scc.rpcData("SPProxySrvAPI.WriteRead", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "WriteRead %v %v %v %v", req.Fd, len(iniov), len(d), err)
	if err == nil && len(outiov) != len(d) {
		return fmt.Errorf("sigmaclntclnt outiov len wrong: supplied %v != %v returned", len(outiov), len(d))
	}
	copy(outiov, d)
	return err
}

func (scc *SPProxyClnt) DirWatch(fd int) (int, error) {
	req := spproto.SigmaReadReq{Fd: uint32(fd)}
	rep := spproto.SigmaFdRep{}
	fd, err := scc.rpcFd("SPProxySrvAPI.DirWatch", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "DirWatch %v %v %v", req, rep, err)
	return fd, err
}

func (scc *SPProxyClnt) MountTree(ep *sp.Tendpoint, tree, mount string) error {
	req := spproto.SigmaMountTreeReq{Endpoint: ep.GetProto(), Tree: tree, MountName: mount}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.MountTree", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "MountTree %v %v %v", req, rep, err)
	return err
}

func (scc *SPProxyClnt) IsLocalMount(ep *sp.Tendpoint) (bool, error) {
	req := spproto.SigmaMountReq{Endpoint: ep.GetProto()}
	rep := spproto.SigmaMountRep{}
	err := scc.rpcc.RPC("SPProxySrvAPI.IsLocalMount", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "IsLocalMount %v %v %v", req, rep, err)
	if err != nil {
		return false, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return false, sp.NewErr(rep.Err)
	}
	return rep.Local, nil
}

func (scc *SPProxyClnt) PathLastMount(pn string) (path.Tpathname, path.Tpathname, error) {
	req := spproto.SigmaPathReq{Path: pn}
	rep := spproto.SigmaLastMountRep{}
	err := scc.rpcc.RPC("SPProxySrvAPI.PathLastMount", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "PathLastMount %v %v %v", req, rep, err)
	if err != nil {
		return nil, nil, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil, nil, sp.NewErr(rep.Err)
	}
	return rep.Path1, rep.Path2, nil
}

func (scc *SPProxyClnt) GetNamedEndpoint() (*sp.Tendpoint, error) {
	return scc.GetNamedEndpointRealm(scc.pe.GetRealm())
}

func (scc *SPProxyClnt) InvalidateNamedEndpointCacheEntryRealm(realm sp.Trealm) error {
	req := spproto.SigmaRealmReq{RealmStr: realm.String()}
	rep := spproto.SigmaMountRep{}
	err := scc.rpcc.RPC("SPProxySrvAPI.InvalidateNamedEndpointCacheEntryRealm", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "InvalidateNamedEndpointCacheEntryRealm %v %v %v", req, rep, err)
	if err != nil {
		return nil
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil
	}
	return nil
}

func (scc *SPProxyClnt) GetNamedEndpointRealm(realm sp.Trealm) (*sp.Tendpoint, error) {
	req := spproto.SigmaRealmReq{RealmStr: realm.String()}
	rep := spproto.SigmaMountRep{}
	err := scc.rpcc.RPC("SPProxySrvAPI.GetNamedEndpointRealm", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "GetNamedEndpointRealm %v %v %v", req, rep, err)
	if err != nil {
		return nil, nil
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil, nil
	}
	return sp.NewEndpointFromProto(rep.Endpoint), nil
}

func (scc *SPProxyClnt) NewRootMount(pn, mntname string) error {
	req := spproto.SigmaMountTreeReq{Tree: pn, MountName: mntname}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.NewRootMount", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "NewRootMount %v %v", req, rep)
	return err
}

func (scc *SPProxyClnt) Mounts() []string {
	req := spproto.SigmaNullReq{}
	rep := spproto.SigmaMountsRep{}
	err := scc.rpcc.RPC("SPProxySrvAPI.Mounts", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Mounts %v %v %v", req, rep, err)
	if err != nil {
		return nil
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return nil
	}
	return rep.Endpoints
}

func (scc *SPProxyClnt) SetLocalMount(ep *sp.Tendpoint, port sp.Tport) {
	db.DFatalf("SetLocalMount %v", ep)
}

func (scc *SPProxyClnt) MountPathClnt(path string, clnt sos.PathClntAPI) error {
	return serr.NewErr(serr.TErrNotSupported, "MountPathClnt")
}

func (scc *SPProxyClnt) Detach(path string) error {
	req := spproto.SigmaPathReq{Path: path}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.Detach", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Detach %v %v %v", req, rep, err)
	return err
}

func (scc *SPProxyClnt) Disconnected() bool {
	return scc.disconnected
}

func (scc *SPProxyClnt) Disconnect(path string) error {
	scc.disconnected = true
	req := spproto.SigmaPathReq{Path: path}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.Disconnect", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Disconnect %v %v %v", req, rep, err)
	return err
}

func (scc *SPProxyClnt) Close() error {
	req := spproto.SigmaNullReq{}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.Close", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Close %v %v %v", req, rep, err)
	scc.close()
	return err
}

// ========== ProcClnt API ==========
func (scc *SPProxyClnt) Started() error {
	req := spproto.SigmaNullReq{}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.Started", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Started %v %v %v", req, rep, err)
	return err
}

func (scc *SPProxyClnt) Exited() error {
	req := spproto.SigmaNullReq{}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.Exited", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Exited %v %v %v", req, rep, err)
	return err
}
