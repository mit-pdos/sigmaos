// The fsclnt package implements the SigmaOS file API but most of the
// heavy lifting is done by [pathclnt].
package fsclnt

import (
	"fmt"
	"io"

	sos "sigmaos/api/sigmaos"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	"sigmaos/sigmaclnt/fidclnt"
	"sigmaos/sigmaclnt/fsclnt/pathclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/syncmap"
)

type FsClient struct {
	pe           *proc.ProcEnv
	pc           *pathclnt.PathClnt
	fds          *FdTable
	ft           *FenceTable
	mnts         *syncmap.SyncMap[string, sos.PathClntAPI]
	disconnected bool
}

func NewFsClient(pe *proc.ProcEnv, fidc *fidclnt.FidClnt) sos.FileAPI {
	fsc := &FsClient{
		pe:   pe,
		pc:   pathclnt.NewPathClnt(pe, fidc),
		fds:  newFdTable(),
		ft:   newFenceTable(),
		mnts: syncmap.NewSyncMap[string, sos.PathClntAPI](),
	}
	fsc.mnts.Insert("name", fsc.pc)
	return fsc
}

func (fsc *FsClient) String() string {
	return fmt.Sprintf("{fsc: realm %v principal %v pathc %v fds %v}", fsc.pe.GetRealm(), fsc.pe.GetPrincipal(), fsc.pc, fsc.fds)
}

func (fsc *FsClient) CloseFd(fd int) error {
	fid, pc, err := fsc.fds.lookup(fd)
	if err != nil {
		return err
	}
	if err := pc.Clunk(fid); err != nil {
		return err
	}
	return nil
}

func (fsc *FsClient) Stat(pn sp.Tsigmapath) (*sp.Tstat, error) {
	return fsc.pc.Stat(pn, fsc.pe.GetPrincipal())
}

func (fsc *FsClient) Create(pn sp.Tsigmapath, perm sp.Tperm, mode sp.Tmode) (int, error) {
	pc, err := fsc.mntLookup(pn)
	if err != nil {
		return -1, err
	}
	f := fsc.ft.lookup(pn)
	fid, err := pc.Create(pn, fsc.pe.GetPrincipal(), perm, mode, sp.NoLeaseId, f)
	if err != nil {
		return -1, err
	}
	fd := fsc.fds.allocFd(fid, mode, pc, pn)
	return fd, nil
}

func (fsc *FsClient) CreateLeased(pn sp.Tsigmapath, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence) (int, error) {
	fid, err := fsc.pc.Create(pn, fsc.pe.GetPrincipal(), perm, mode, lid, &f)
	if err != nil {
		return -1, err
	}
	fd := fsc.fds.allocFd(fid, mode, fsc.pc, pn)
	return fd, nil
}

func (fsc *FsClient) openWait(pc sos.PathClntAPI, pn sp.Tsigmapath, mode sp.Tmode) (int, error) {
	ch := make(chan error)
	fd := -1
	for {
		fid, err := pc.Open(pn, fsc.pe.GetPrincipal(), mode, func(err error) {
			ch <- err
		})
		if serr.IsErrCode(err, serr.TErrNotfound) {
			db.DPrintf(db.FSCLNT, "openWatch wait %v\n", pn)
			r := <-ch
			if r != nil {
				db.DPrintf(db.FSCLNT, "Open watch wait %v err %v\n", pn, err)
			}
		} else if err != nil {
			db.DPrintf(db.FSCLNT, "openWatch %v err %v\n", pn, err)
			return -1, err
		} else { // success; file is opened
			fd = fsc.fds.allocFd(fid, mode, pc, pn)
			break
		}
	}
	return fd, nil
}

func (fsc *FsClient) Open(pn sp.Tsigmapath, mode sp.Tmode, w sos.Twait) (int, error) {
	pc, err := fsc.mntLookup(pn)
	if err != nil {
		return -1, err
	}
	if w {
		return fsc.openWait(pc, pn, mode)
	} else {
		fid, err := pc.Open(pn, fsc.pe.GetPrincipal(), mode, nil)
		if err != nil {
			return -1, err
		}
		fd := fsc.fds.allocFd(fid, mode, pc, pn)
		return fd, nil
	}
}

func (fsc *FsClient) Rename(old, new sp.Tsigmapath) error {
	f := fsc.ft.lookup(old)
	return fsc.pc.Rename(old, new, fsc.pe.GetPrincipal(), f)
}

func (fsc *FsClient) Remove(pn sp.Tsigmapath) error {
	f := fsc.ft.lookup(pn)
	return fsc.pc.Remove(pn, fsc.pe.GetPrincipal(), f)
}

func (fsc *FsClient) GetFile(pn sp.Tsigmapath) ([]byte, error) {
	f := fsc.ft.lookup(pn)
	return fsc.pc.GetFile(pn, fsc.pe.GetPrincipal(), sp.OREAD, 0, sp.MAXGETSET, f)
}

func (fsc *FsClient) PutFile(pn string, perm sp.Tperm, mode sp.Tmode, data []byte, off sp.Toffset, lid sp.TleaseId) (sp.Tsize, error) {
	f := fsc.ft.lookup(pn)
	return fsc.pc.PutFile(pn, fsc.pe.GetPrincipal(), mode|sp.OWRITE, perm, data, off, lid, f)
}

func (fsc *FsClient) readFid(fd int, pc sos.PathClntAPI, fid sp.Tfid, off sp.Toffset, b []byte) (sp.Tsize, error) {
	cnt, err := pc.ReadF(fid, off, b, sp.NullFence())
	if err != nil {
		return 0, err
	}
	return cnt, nil
}

func (fsc *FsClient) readFidRdr(fd int, pc sos.PathClntAPI, fid sp.Tfid, off sp.Toffset, sz sp.Tsize) (io.ReadCloser, error) {
	rdr, err := pc.PreadRdr(fid, off, sz)
	if err != nil {
		return nil, err
	}
	return rdr, nil
}

func (fsc *FsClient) Read(fd int, b []byte) (sp.Tsize, error) {
	fid, off, pc, sr := fsc.fds.lookupOff(fd)
	if sr != nil {
		return 0, sr
	}
	cnt, err := fsc.readFid(fd, pc, fid, off, b)
	if err != nil {
		return 0, err
	}
	fsc.fds.incOff(fd, sp.Toffset(cnt))
	return cnt, nil
}

func (fsc *FsClient) Pread(fd int, b []byte, o sp.Toffset) (sp.Tsize, error) {
	fid, _, pc, sr := fsc.fds.lookupOff(fd)
	if sr != nil {
		return 0, sr
	}
	return fsc.readFid(fd, pc, fid, o, b)
}

func (fsc *FsClient) PreadRdr(fd int, o sp.Toffset, l sp.Tsize) (io.ReadCloser, error) {
	fid, _, pc, sr := fsc.fds.lookupOff(fd)
	if sr != nil {
		return nil, sr
	}
	return fsc.readFidRdr(fd, pc, fid, o, l)
}

func (fsc *FsClient) writeFid(fd int, pc sos.PathClntAPI, fid sp.Tfid, off sp.Toffset, data []byte, f0 sp.Tfence) (sp.Tsize, error) {
	f := &f0
	if !f0.HasFence() {
		pn, err := fsc.fds.lookupPn(fd)
		if err != nil {
			return 0, err
		}
		f = fsc.ft.lookup(pn)
	}
	sz, err := pc.WriteF(fid, off, data, f)
	if err != nil {
		return 0, err
	}
	fsc.fds.incOff(fd, sp.Toffset(sz))
	return sz, nil
}

func (fsc *FsClient) Write(fd int, data []byte) (sp.Tsize, error) {
	fid, off, pc, err := fsc.fds.lookupOff(fd)
	if err != nil {
		return 0, err
	}
	return fsc.writeFid(fd, pc, fid, off, data, sp.NoFence())
}

func (fsc *FsClient) WriteFence(fd int, data []byte, f sp.Tfence) (sp.Tsize, error) {
	fid, off, _, err := fsc.fds.lookupOff(fd)
	if err != nil {
		return 0, err
	}
	return fsc.writeFid(fd, fsc.pc, fid, off, data, f)
}

func (fsc *FsClient) WriteRead(fd int, iniov sessp.IoVec, outiov sessp.IoVec) error {
	fid, _, _, err := fsc.fds.lookupOff(fd)
	if err != nil {
		return err
	}
	if err := fsc.pc.WriteRead(fid, iniov, outiov); err != nil {
		return err
	}
	return nil
}

func (fsc *FsClient) Seek(fd int, off sp.Toffset) error {
	err := fsc.fds.setOffset(fd, off)
	if err != nil {
		return err
	}
	return nil
}

func (fsc *FsClient) DirWatch(fd int) (int, error) {
	fid, _, err := fsc.fds.lookup(fd)
	if err != nil {
		return -1, err
	}
	db.DPrintf(db.FSCLNT, "DirWatch: dir fd %v dir fid %d\n", fd, fid)
	watchfid, err2 := fsc.pc.SetDirWatch(fid)
	if err2 != nil {
		db.DPrintf(db.FSCLNT, "SetDirWatch err %v\n", err)
		return -1, err2
	}

	watchfd := fsc.fds.allocFd(watchfid, sp.OREAD, fsc.pc, "")
	db.DPrintf(db.FSCLNT, "DirWatch: watch fd %v watch fid %d\n", watchfd, watchfid)

	return watchfd, nil
}

func (fsc *FsClient) IsLocalMount(ep *sp.Tendpoint) (bool, error) {
	return fsc.pc.IsLocalMount(ep)
}

func (fsc *FsClient) SetLocalMount(ep *sp.Tendpoint, port sp.Tport) {
	ep.SetAddr([]*sp.Taddr{sp.NewTaddr(fsc.pe.GetInnerContainerIP(), port)})
}

func (fsc *FsClient) PathLastMount(pn sp.Tsigmapath) (path.Tpathname, path.Tpathname, error) {
	return fsc.pc.MntClnt().PathLastMount(pn, fsc.pe.GetPrincipal())
}

func (fsc *FsClient) MountTree(ep *sp.Tendpoint, tree, mount sp.Tsigmapath) error {
	return fsc.pc.MntClnt().MountTree(fsc.pe.GetSecrets(), ep, tree, mount)
}

func (fsc *FsClient) GetNamedEndpoint() (*sp.Tendpoint, error) {
	return fsc.GetNamedEndpointRealm(fsc.pe.GetRealm())
}

func (fsc *FsClient) InvalidateNamedEndpointCacheEntryRealm(realm sp.Trealm) error {
	return fsc.pc.MntClnt().InvalidateNamedEndpointCacheEntryRealm(realm)
}

func (fsc *FsClient) GetNamedEndpointRealm(realm sp.Trealm) (*sp.Tendpoint, error) {
	return fsc.pc.MntClnt().GetNamedEndpointRealm(realm)
}

func (fsc *FsClient) NewRootMount(pn, epname sp.Tsigmapath) error {
	return fsc.pc.MntClnt().NewRootMount(fsc.pe.GetPrincipal(), pn, epname)
}

func (fsc *FsClient) MountPathClnt(mnt sp.Tsigmapath, clnt sos.PathClntAPI) error {
	fsc.mnts.Insert(mnt, clnt)
	return nil
}

func (fsc *FsClient) Mounts() []sp.Tsigmapath {
	return fsc.pc.MntClnt().MountedPaths()
}

func (fsc *FsClient) ClntId() sp.TclntId {
	return fsc.pc.ClntId()
}

func (fsc *FsClient) FenceDir(pn sp.Tsigmapath, fence sp.Tfence) error {
	return fsc.ft.insert(pn, fence)
}

func (fsc *FsClient) Disconnected() bool {
	return fsc.disconnected
}

func (fsc *FsClient) Disconnect(pn sp.Tsigmapath) error {
	fsc.disconnected = true
	return fsc.pc.Disconnect(pn)
}

func (fsc *FsClient) Detach(pn sp.Tsigmapath) error {
	return fsc.pc.MntClnt().Detach(pn)
}

func (fsc *FsClient) Close() error {
	return fsc.pc.Close()
}

func (fsc *FsClient) Stats() (*sos.ClntStats, error) {
	spst := fsc.pc.FidClnt.PathClntStats().StatsSnapshot()
	spst.Counters["Nfid"] = int64(fsc.pc.FidClnt.Len())
	st := &sos.ClntStats{
		Path: spst,
		Sp:   fsc.pc.FidClnt.SpStats().StatsSnapshot(),
	}
	return st, nil
}

func (fsc *FsClient) mntLookup(pn sp.Tsigmapath) (sos.PathClntAPI, error) {
	p := path.Split(pn)
	if len(p) == 0 {
		return nil, serr.NewErr(serr.TErrInval, pn[0])
	}
	pc, ok := fsc.mnts.Lookup(p[0])
	if !ok {
		return nil, serr.NewErr(serr.TErrNotfound, p[0])
	}
	return pc, nil
}
