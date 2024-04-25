// Fdclnt package implements the SigmaOS API but most of the heavy
// lifting is done by [pathclnt].
package fdclnt

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/path"
	"sigmaos/pathclnt"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessp"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
)

type FdClient struct {
	pe           *proc.ProcEnv
	pc           *pathclnt.PathClnt
	fds          *FdTable
	ft           *FenceTable
	disconnected bool
}

func NewFdClient(pe *proc.ProcEnv, fsc *fidclnt.FidClnt) sos.SigmaOS {
	fdc := &FdClient{pe: pe}
	fdc.pc = pathclnt.NewPathClnt(pe, fsc)
	fdc.fds = newFdTable()
	fdc.ft = newFenceTable()
	return fdc
}

func (fdc *FdClient) String() string {
	str := fmt.Sprintf("Table:\n")
	str += fmt.Sprintf("fds %v\n", fdc.fds)
	str += fmt.Sprintf("fsc %v\n", fdc.pc)
	return str
}

func (fdc *FdClient) CloseFd(fd int) error {
	fid, error := fdc.fds.lookup(fd)
	if error != nil {
		return error
	}
	if err := fdc.pc.Clunk(fid); err != nil {
		return err
	}
	return nil
}

func (fdc *FdClient) Stat(name string) (*sp.Stat, error) {
	return fdc.pc.Stat(name, fdc.pe.GetPrincipal())
}

func (fdc *FdClient) Create(path string, perm sp.Tperm, mode sp.Tmode) (int, error) {
	fid, err := fdc.pc.Create(path, fdc.pe.GetPrincipal(), perm, mode, sp.NoLeaseId, sp.NoFence())
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) CreateEphemeral(path string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence) (int, error) {
	fid, err := fdc.pc.Create(path, fdc.pe.GetPrincipal(), perm|sp.DMTMP, mode, lid, f)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) openWait(path string, mode sp.Tmode) (int, error) {
	ch := make(chan error)
	fd := -1
	for {
		fid, err := fdc.pc.Open(path, fdc.pe.GetPrincipal(), mode, func(err error) {
			ch <- err
		})
		db.DPrintf(db.FDCLNT, "openWatch %v err %v\n", path, err)
		if serr.IsErrCode(err, serr.TErrNotfound) {
			r := <-ch
			if r != nil {
				db.DPrintf(db.FDCLNT, "Open watch %v err %v\n", path, err)
			}
		} else if err != nil {
			return -1, err
		} else { // success; file is opened
			fd = fdc.fds.allocFd(fid, mode)
			break
		}
	}
	return fd, nil
}

func (fdc *FdClient) Open(path string, mode sp.Tmode, w sos.Twait) (int, error) {
	if w {
		return fdc.openWait(path, mode)
	} else {
		fid, err := fdc.pc.Open(path, fdc.pe.GetPrincipal(), mode, nil)
		if err != nil {
			return -1, err
		}
		fd := fdc.fds.allocFd(fid, mode)
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

func (fdc *FdClient) readFid(fd int, fid sp.Tfid, off sp.Toffset, b []byte) (sp.Tsize, error) {
	cnt, err := fdc.pc.ReadF(fid, off, b, sp.NullFence())
	if err != nil {
		return 0, err
	}
	return cnt, nil
}

func (fdc *FdClient) Read(fd int, b []byte) (sp.Tsize, error) {
	fid, off, sr := fdc.fds.lookupOff(fd)
	if sr != nil {
		return 0, sr
	}
	cnt, err := fdc.readFid(fd, fid, off, b)
	if err != nil {
		return 0, err
	}
	fdc.fds.incOff(fd, sp.Toffset(cnt))
	return cnt, nil
}

func (fdc *FdClient) Pread(fd int, b []byte, o sp.Toffset) (sp.Tsize, error) {
	fid, _, sr := fdc.fds.lookupOff(fd)
	if sr != nil {
		return 0, sr
	}
	return fdc.readFid(fd, fid, o, b)
}

func (fdc *FdClient) writeFid(fd int, fid sp.Tfid, off sp.Toffset, data []byte, f0 sp.Tfence) (sp.Tsize, error) {
	f := &f0
	if !f0.HasFence() {
		ch := fdc.pc.FidClnt.Lookup(fid)
		if ch == nil {
			return 0, serr.NewErr(serr.TErrUnreachable, "writeFid")
		}
		f = fdc.ft.lookupPath(ch.Path())
	}
	sz, err := fdc.pc.WriteF(fid, off, data, f)
	if err != nil {
		return 0, err
	}
	fdc.fds.incOff(fd, sp.Toffset(sz))
	return sz, nil
}

func (fdc *FdClient) Write(fd int, data []byte) (sp.Tsize, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return 0, error
	}
	return fdc.writeFid(fd, fid, off, data, sp.NoFence())
}

func (fdc *FdClient) WriteFence(fd int, data []byte, f sp.Tfence) (sp.Tsize, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return 0, error
	}
	return fdc.writeFid(fd, fid, off, data, f)
}

func (fdc *FdClient) WriteRead(fd int, iniov sessp.IoVec, outiov sessp.IoVec) error {
	fid, _, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return error
	}
	err := fdc.pc.WriteRead(fid, iniov, outiov)
	if err != nil {
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

func (fdc *FdClient) DirWait(fd int) error {
	fid, err := fdc.fds.lookup(fd)
	if err != nil {
		return err
	}
	db.DPrintf(db.FDCLNT, "DirWait: watch fd %v\n", fd)
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

func (fdc *FdClient) PathLastMount(pn string) (path.Path, path.Path, error) {
	return fdc.pc.PathLastMount(pn, fdc.pe.GetPrincipal())
}

func (fdc *FdClient) MountTree(ep *sp.Tendpoint, tree, mount string) error {
	return fdc.pc.MountTree(fdc.pe.GetPrincipal(), ep, tree, mount)
}

func (fdc *FdClient) GetNamedEndpoint() (*sp.Tendpoint, error) {
	return fdc.pc.GetNamedEndpoint()
}

func (fdc *FdClient) NewRootMount(pn, epname string) error {
	return fdc.pc.NewRootMount(fdc.pe.GetPrincipal(), pn, epname)
}

func (fdc *FdClient) Mounts() []string {
	return fdc.pc.Mounts()
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
	fids := fdc.fds.openfids()
	return fdc.pc.Disconnect(pn, fids)
}

func (fdc *FdClient) Detach(pn string) error {
	return fdc.pc.Detach(pn)
}

func (fdc *FdClient) Close() error {
	return fdc.pc.Close()
}
