// Fdclnt package implements the SigmaOS API but most of the heavy
// lifting is done by [pathclnt].
package fdclnt

import (
	"fmt"
	"io"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessp"
	"sigmaos/sigmaclnt/fdclnt/pathclnt"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
	"sigmaos/util/syncmap"
)

type FdClient struct {
	pe           *proc.ProcEnv
	pc           *pathclnt.PathClnt
	fds          *FdTable
	ft           *FenceTable
	mnts         *syncmap.SyncMap[string, sos.PathClntAPI]
	disconnected bool
}

func NewFdClient(pe *proc.ProcEnv, fsc *fidclnt.FidClnt) sos.FileAPI {
	fdc := &FdClient{
		pe:   pe,
		pc:   pathclnt.NewPathClnt(pe, fsc),
		fds:  newFdTable(),
		ft:   newFenceTable(),
		mnts: syncmap.NewSyncMap[string, sos.PathClntAPI](),
	}
	fdc.mnts.Insert("name", fdc.pc)
	return fdc
}

func (fdc *FdClient) String() string {
	return fmt.Sprintf("{fdc: realm %v principal %v pathc %v fds %v}", fdc.pe.GetRealm(), fdc.pe.GetPrincipal(), fdc.pc, fdc.fds)
}

func (fdc *FdClient) CloseFd(fd int) error {
	fid, pc, err := fdc.fds.lookup(fd)
	if err != nil {
		return err
	}
	if err := pc.Clunk(fid); err != nil {
		return err
	}
	return nil
}

func (fdc *FdClient) Stat(name string) (*sp.Tstat, error) {
	return fdc.pc.Stat(name, fdc.pe.GetPrincipal())
}

func (fdc *FdClient) Create(pn string, perm sp.Tperm, mode sp.Tmode) (int, error) {
	pc, err := fdc.mntLookup(pn)
	if err != nil {
		return -1, err
	}
	f := fdc.ft.lookup(pn)
	fid, err := pc.Create(pn, fdc.pe.GetPrincipal(), perm, mode, sp.NoLeaseId, f)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode, pc, pn)
	return fd, nil
}

func (fdc *FdClient) CreateLeased(pn string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence) (int, error) {
	fid, err := fdc.pc.Create(pn, fdc.pe.GetPrincipal(), perm, mode, lid, &f)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode, fdc.pc, pn)
	return fd, nil
}

func (fdc *FdClient) openWait(pc sos.PathClntAPI, pn string, mode sp.Tmode) (int, error) {
	ch := make(chan error)
	fd := -1
	for {
		fid, err := pc.Open(pn, fdc.pe.GetPrincipal(), mode, func(err error) {
			ch <- err
		})
		if serr.IsErrCode(err, serr.TErrNotfound) {
			db.DPrintf(db.FDCLNT, "openWatch wait %v\n", pn)
			r := <-ch
			if r != nil {
				db.DPrintf(db.FDCLNT, "Open watch wait %v err %v\n", pn, err)
			}
		} else if err != nil {
			db.DPrintf(db.FDCLNT, "openWatch %v err %v\n", pn, err)
			return -1, err
		} else { // success; file is opened
			fd = fdc.fds.allocFd(fid, mode, pc, pn)
			break
		}
	}
	return fd, nil
}

func (fdc *FdClient) Open(pn string, mode sp.Tmode, w sos.Twait) (int, error) {
	pc, err := fdc.mntLookup(pn)
	if err != nil {
		return -1, err
	}
	if w {
		return fdc.openWait(pc, pn, mode)
	} else {
		fid, err := pc.Open(pn, fdc.pe.GetPrincipal(), mode, nil)
		if err != nil {
			return -1, err
		}
		fd := fdc.fds.allocFd(fid, mode, pc, pn)
		return fd, nil
	}
}

func (fdc *FdClient) Rename(old, new string) error {
	f := fdc.ft.lookup(old)
	return fdc.pc.Rename(old, new, fdc.pe.GetPrincipal(), f)
}

func (fdc *FdClient) Remove(pn string) error {
	f := fdc.ft.lookup(pn)
	return fdc.pc.Remove(pn, fdc.pe.GetPrincipal(), f)
}

func (fdc *FdClient) GetFile(pn string) ([]byte, error) {
	f := fdc.ft.lookup(pn)
	return fdc.pc.GetFile(pn, fdc.pe.GetPrincipal(), sp.OREAD, 0, sp.MAXGETSET, f)
}

func (fdc *FdClient) PutFile(fname string, perm sp.Tperm, mode sp.Tmode, data []byte, off sp.Toffset, lid sp.TleaseId) (sp.Tsize, error) {
	f := fdc.ft.lookup(fname)
	return fdc.pc.PutFile(fname, fdc.pe.GetPrincipal(), mode|sp.OWRITE, perm, data, off, lid, f)
}

func (fdc *FdClient) readFid(fd int, pc sos.PathClntAPI, fid sp.Tfid, off sp.Toffset, b []byte) (sp.Tsize, error) {
	cnt, err := pc.ReadF(fid, off, b, sp.NullFence())
	if err != nil {
		return 0, err
	}
	return cnt, nil
}

func (fdc *FdClient) readFidRdr(fd int, pc sos.PathClntAPI, fid sp.Tfid, off sp.Toffset, sz sp.Tsize) (io.ReadCloser, error) {
	rdr, err := pc.PreadRdr(fid, off, sz)
	if err != nil {
		return nil, err
	}
	return rdr, nil
}

func (fdc *FdClient) Read(fd int, b []byte) (sp.Tsize, error) {
	fid, off, pc, sr := fdc.fds.lookupOff(fd)
	if sr != nil {
		return 0, sr
	}
	cnt, err := fdc.readFid(fd, pc, fid, off, b)
	if err != nil {
		return 0, err
	}
	fdc.fds.incOff(fd, sp.Toffset(cnt))
	return cnt, nil
}

func (fdc *FdClient) Pread(fd int, b []byte, o sp.Toffset) (sp.Tsize, error) {
	fid, _, pc, sr := fdc.fds.lookupOff(fd)
	if sr != nil {
		return 0, sr
	}
	return fdc.readFid(fd, pc, fid, o, b)
}

func (fdc *FdClient) PreadRdr(fd int, o sp.Toffset, l sp.Tsize) (io.ReadCloser, error) {
	fid, _, pc, sr := fdc.fds.lookupOff(fd)
	if sr != nil {
		return nil, sr
	}
	return fdc.readFidRdr(fd, pc, fid, o, l)
}

func (fdc *FdClient) writeFid(fd int, pc sos.PathClntAPI, fid sp.Tfid, off sp.Toffset, data []byte, f0 sp.Tfence) (sp.Tsize, error) {
	f := &f0
	if !f0.HasFence() {
		pn, err := fdc.fds.lookupPn(fd)
		if err != nil {
			return 0, err
		}
		f = fdc.ft.lookup(pn)
	}
	sz, err := pc.WriteF(fid, off, data, f)
	if err != nil {
		return 0, err
	}
	fdc.fds.incOff(fd, sp.Toffset(sz))
	return sz, nil
}

func (fdc *FdClient) Write(fd int, data []byte) (sp.Tsize, error) {
	fid, off, pc, err := fdc.fds.lookupOff(fd)
	if err != nil {
		return 0, err
	}
	return fdc.writeFid(fd, pc, fid, off, data, sp.NoFence())
}

func (fdc *FdClient) WriteFence(fd int, data []byte, f sp.Tfence) (sp.Tsize, error) {
	fid, off, _, err := fdc.fds.lookupOff(fd)
	if err != nil {
		return 0, err
	}
	return fdc.writeFid(fd, fdc.pc, fid, off, data, f)
}

func (fdc *FdClient) WriteRead(fd int, iniov sessp.IoVec, outiov sessp.IoVec) error {
	fid, _, _, err := fdc.fds.lookupOff(fd)
	if err != nil {
		return err
	}
	if err := fdc.pc.WriteRead(fid, iniov, outiov); err != nil {
		return err
	}
	return nil
}

func (fdc *FdClient) Seek(fd int, off sp.Toffset) error {
	err := fdc.fds.setOffset(fd, off)
	if err != nil {
		return err
	}
	return nil
}

func (fdc *FdClient) DirWatch(fd int) error {
	fid, _, err := fdc.fds.lookup(fd)
	if err != nil {
		return err
	}
	db.DPrintf(db.FDCLNT, "DirWatch: watch fd %v\n", fd)
	ch := make(chan error)
	if err := fdc.pc.SetDirWatch(fid, func(r error) {
		db.DPrintf(db.FDCLNT, "SetDirWatch: watch returns %v\n", r)
		ch <- r
	}); err != nil {
		db.DPrintf(db.FDCLNT, "SetDirWatch err %v\n", err)
		return err
	}
	if err := <-ch; err != nil {
		return err
	}
	return nil
}

func (fdc *FdClient) IsLocalMount(ep *sp.Tendpoint) (bool, error) {
	return fdc.pc.IsLocalMount(ep)
}

func (fdc *FdClient) SetLocalMount(ep *sp.Tendpoint, port sp.Tport) {
	ep.SetAddr([]*sp.Taddr{sp.NewTaddr(fdc.pe.GetInnerContainerIP(), sp.INNER_CONTAINER_IP, port)})
}

func (fdc *FdClient) PathLastMount(pn string) (path.Tpathname, path.Tpathname, error) {
	return fdc.pc.MntClnt().PathLastMount(pn, fdc.pe.GetPrincipal())
}

func (fdc *FdClient) MountTree(ep *sp.Tendpoint, tree, mount string) error {
	return fdc.pc.MntClnt().MountTree(fdc.pe.GetSecrets(), ep, tree, mount)
}

func (fdc *FdClient) GetNamedEndpoint() (*sp.Tendpoint, error) {
	return fdc.GetNamedEndpointRealm(fdc.pe.GetRealm())
}

func (fdc *FdClient) InvalidateNamedEndpointCacheEntryRealm(realm sp.Trealm) error {
	return fdc.pc.MntClnt().InvalidateNamedEndpointCacheEntryRealm(realm)
}

func (fdc *FdClient) GetNamedEndpointRealm(realm sp.Trealm) (*sp.Tendpoint, error) {
	return fdc.pc.MntClnt().GetNamedEndpointRealm(realm)
}

func (fdc *FdClient) NewRootMount(pn, epname string) error {
	return fdc.pc.MntClnt().NewRootMount(fdc.pe.GetPrincipal(), pn, epname)
}

func (fdc *FdClient) MountPathClnt(mnt string, clnt sos.PathClntAPI) error {
	fdc.mnts.Insert(mnt, clnt)
	return nil
}

func (fdc *FdClient) Mounts() []string {
	return fdc.pc.MntClnt().MountedPaths()
}

func (fdc *FdClient) ClntId() sp.TclntId {
	return fdc.pc.ClntId()
}

func (fdc *FdClient) FenceDir(pn string, fence sp.Tfence) error {
	return fdc.ft.insert(pn, fence)
}

func (fdc *FdClient) Disconnected() bool {
	return fdc.disconnected
}

func (fdc *FdClient) Disconnect(pn string) error {
	fdc.disconnected = true
	return fdc.pc.Disconnect(pn)
}

func (fdc *FdClient) Detach(pn string) error {
	return fdc.pc.MntClnt().Detach(pn)
}

func (fdc *FdClient) Close() error {
	return fdc.pc.Close()
}

func (fdc *FdClient) mntLookup(pn string) (sos.PathClntAPI, error) {
	p := path.Split(pn)
	if len(p) == 0 {
		return nil, serr.NewErr(serr.TErrInval, pn[0])
	}
	pc, ok := fdc.mnts.Lookup(p[0])
	if !ok {
		return nil, serr.NewErr(serr.TErrNotfound, p[0])
	}
	return pc, nil
}
