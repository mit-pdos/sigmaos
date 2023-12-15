package fdclnt

//
// Fdclnt package implements the SigmaOS API
//

import (
	"fmt"
	"net"

	"sigmaos/fidclnt"
	"sigmaos/path"
	"sigmaos/pathclnt"
	"sigmaos/proc"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
)

//
// Procs interact with servers using Unix-like file descriptor
// interface and pathnames.
//
// A hypothetical kernel could multiplex multiple procs over one
// FidClnt, which allows a shared TCP connection to a server. A kernel
// could also use fds to share file descriptors state (e.g., offset)
// between parent and child.  Since we have no kernel implementing
// procs, these use cases are speculative.
//
// The FdClient is per user, while a single pathclnt can be shared
// between many FdClients since pathclnt requires a uname being passed
// in. The standard use case is, however, to have one pathclnt per
// FdClient.
//

type FdClient struct {
	pcfg *proc.ProcEnv
	pc   *pathclnt.PathClnt
	fds  *FdTable
}

func NewFdClient(pcfg *proc.ProcEnv, fsc *fidclnt.FidClnt) sos.SigmaOS {
	fdc := &FdClient{pcfg: pcfg}
	fdc.pc = pathclnt.NewPathClnt(pcfg, fsc)
	fdc.fds = newFdTable()
	return fdc
}

func (fdc *FdClient) String() string {
	str := fmt.Sprintf("Table:\n")
	str += fmt.Sprintf("fds %v\n", fdc.fds)
	str += fmt.Sprintf("fsc %v\n", fdc.pc)
	return str
}

func (fdc *FdClient) Close(fd int) error {
	fid, error := fdc.fds.lookup(fd)
	if error != nil {
		return error
	}
	err := fdc.pc.Clunk(fid)
	if err != nil {
		return err
	}
	return nil
}

func (fdc *FdClient) Qid(fd int) (*sp.Tqid, error) {
	fid, error := fdc.fds.lookup(fd)
	if error != nil {
		return nil, error
	}
	return fdc.pc.Qid(fid), nil
}

func (fdc *FdClient) Stat(name string) (*sp.Stat, error) {
	return fdc.pc.Stat(name, fdc.pcfg.GetUname())
}

func (fdc *FdClient) Create(path string, perm sp.Tperm, mode sp.Tmode) (int, error) {
	fid, err := fdc.pc.Create(path, fdc.pcfg.GetUname(), perm, mode, sp.NoLeaseId, sp.NoFence())
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) CreateEphemeral(path string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence) (int, error) {
	fid, err := fdc.pc.Create(path, fdc.pcfg.GetUname(), perm|sp.DMTMP, mode, lid, f)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) OpenWatch(path string, mode sp.Tmode, w sos.Watch) (int, error) {
	fid, err := fdc.pc.OpenWatch(path, fdc.pcfg.GetUname(), mode, w)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) Open(path string, mode sp.Tmode) (int, error) {
	return fdc.OpenWatch(path, mode, nil)
}

func (fdc *FdClient) SetRemoveWatch(pn string, w sos.Watch) error {
	return fdc.pc.SetRemoveWatch(pn, fdc.pcfg.GetUname(), w)
}

func (fdc *FdClient) Rename(old, new string) error {
	return fdc.pc.Rename(old, new, fdc.pcfg.GetUname())
}

func (fdc *FdClient) Remove(pn string) error {
	return fdc.pc.Remove(pn, fdc.pcfg.GetUname())
}

func (fdc *FdClient) GetFile(fname string) ([]byte, error) {
	return fdc.pc.GetFile(fname, fdc.pcfg.GetUname(), sp.OREAD, 0, sp.MAXGETSET)
}

func (fdc *FdClient) PutFile(fname string, perm sp.Tperm, mode sp.Tmode, data []byte, off sp.Toffset, lid sp.TleaseId) (sp.Tsize, error) {
	return fdc.pc.PutFile(fname, fdc.pcfg.GetUname(), mode|sp.OWRITE, perm, data, off, lid)
}

func (fdc *FdClient) readFid(fd int, fid sp.Tfid, off sp.Toffset, cnt sp.Tsize) ([]byte, error) {
	data, err := fdc.pc.ReadF(fid, off, cnt)
	if err != nil {
		return nil, err
	}
	fdc.fds.incOff(fd, sp.Toffset(len(data)))
	return data, nil
}

func (fdc *FdClient) Read(fd int, cnt sp.Tsize) ([]byte, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return nil, error
	}
	return fdc.readFid(fd, fid, off, cnt)
}

func (fdc *FdClient) writeFid(fd int, fid sp.Tfid, off sp.Toffset, data []byte, f sp.Tfence) (sp.Tsize, error) {
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

func (fdc *FdClient) WriteRead(fd int, data []byte) ([]byte, error) {
	fid, _, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return nil, error
	}
	b, err := fdc.pc.WriteRead(fid, data)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (fdc *FdClient) Seek(fd int, off sp.Toffset) error {
	err := fdc.fds.setOffset(fd, off)
	if err != nil {
		return err
	}
	return nil
}

func (fdc *FdClient) SetDirWatch(fd int, pn string, w sos.Watch) error {
	fid, error := fdc.fds.lookup(fd)
	if error != nil {
		return error
	}
	return fdc.pc.SetDirWatch(fid, pn, w)
}

func (fdc *FdClient) IsLocalMount(mnt sp.Tmount) bool {
	return fdc.pc.IsLocalMount(mnt)
}

func (fdc *FdClient) SetLocalMount(mnt *sp.Tmount, port string) {
	a := net.JoinHostPort(fdc.pcfg.GetLocalIP(), port)
	mnt.SetAddr(sp.NewTaddrs([]string{a}))
}

func (fdc *FdClient) PathLastSymlink(pn string) (path.Path, path.Path, error) {
	return fdc.pc.PathLastSymlink(pn, fdc.pcfg.GetUname())
}

func (fdc *FdClient) MountTree(addrs sp.Taddrs, tree, mount string) error {
	return fdc.pc.MountTree(fdc.pcfg.GetUname(), addrs, tree, mount)
}

func (fdc *FdClient) GetNamedMount() sp.Tmount {
	return fdc.pc.GetNamedMount()
}

func (fdc *FdClient) NewRootMount(pn, mntname string) error {
	return fdc.pc.NewRootMount(fdc.pcfg.GetUname(), pn, mntname)
}

func (fdc *FdClient) Mounts() []string {
	return fdc.pc.Mounts()
}

func (fdc *FdClient) ClntId() sp.TclntId {
	return fdc.pc.ClntId()
}

func (fdc *FdClient) FenceDir(pn string, fence sp.Tfence) error {
	if err := fdc.pc.FenceDir(pn, fence); err != nil {
		return err
	}
	return nil
}

func (fdc *FdClient) Disconnect(pn string) error {
	return fdc.pc.Disconnect(pn)
}

func (fdc *FdClient) Detach(pn string) error {
	return fdc.pc.Detach(pn)
}

func (fdc *FdClient) DetachAll() error {
	return fdc.pc.DetachAll()
}
