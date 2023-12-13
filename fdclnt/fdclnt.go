package fdclnt

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/path"
	"sigmaos/pathclnt"
	"sigmaos/proc"
	"sigmaos/reader"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/writer"
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
	*pathclnt.PathClnt
	fds *FdTable
}

func NewFdClient(pcfg *proc.ProcEnv, fsc *fidclnt.FidClnt) *FdClient {
	fdc := &FdClient{pcfg: pcfg}
	fdc.PathClnt = pathclnt.NewPathClnt(pcfg, fsc)
	fdc.fds = newFdTable()
	return fdc
}

func (fdc *FdClient) String() string {
	str := fmt.Sprintf("Table:\n")
	str += fmt.Sprintf("fds %v\n", fdc.fds)
	str += fmt.Sprintf("fsc %v\n", fdc.PathClnt)
	return str
}

func (fdc *FdClient) Close(fd int) error {
	db.DPrintf(db.ALWAYS, "fdclient got close req")
	fid, error := fdc.fds.lookup(fd)
	if error != nil {
		return error
	}
	err := fdc.PathClnt.Clunk(fid)
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
	return fdc.PathClnt.Qid(fid), nil
}

func (fdc *FdClient) Stat(name string) (*sp.Stat, error) {
	return fdc.PathClnt.Stat(name, fdc.pcfg.GetUname())
}

func (fdc *FdClient) Create(path string, perm sp.Tperm, mode sp.Tmode) (int, error) {
	fid, err := fdc.PathClnt.Create(path, fdc.pcfg.GetUname(), perm, mode, sp.NoLeaseId, sp.NoFence())
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) CreateEphemeral(path string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence) (int, error) {
	fid, err := fdc.PathClnt.Create(path, fdc.pcfg.GetUname(), perm|sp.DMTMP, mode, lid, f)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) OpenWatch(path string, mode sp.Tmode, w pathclnt.Watch) (int, error) {
	fid, err := fdc.PathClnt.OpenWatch(path, fdc.pcfg.GetUname(), mode, w)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) Open(path string, mode sp.Tmode) (int, error) {
	return fdc.OpenWatch(path, mode, nil)
}

func (fdc *FdClient) CreateOpen(path string, perm sp.Tperm, mode sp.Tmode) (int, error) {
	fd, err := fdc.Create(path, perm, mode)
	if err != nil && !serr.IsErrCode(err, serr.TErrExists) {
		db.DPrintf(db.FDCLNT_ERR, "Create %v err %v", path, err)
		return -1, err
	}
	if err != nil {
		fd, err = fdc.Open(path, mode)
		if err != nil {
			db.DPrintf(db.FDCLNT_ERR, "Open %v err %v", path, err)
			return -1, err
		}
	}
	return fd, nil
}

func (fdc *FdClient) SetRemoveWatch(pn string, w pathclnt.Watch) error {
	return fdc.PathClnt.SetRemoveWatch(pn, fdc.pcfg.GetUname(), w)
}

func (fdc *FdClient) Rename(old, new string) error {
	return fdc.PathClnt.Rename(old, new, fdc.pcfg.GetUname())
}

func (fdc *FdClient) Remove(pn string) error {
	return fdc.PathClnt.Remove(pn, fdc.pcfg.GetUname())
}

func (fdc *FdClient) GetFile(fname string) ([]byte, error) {
	return fdc.PathClnt.GetFile(fname, fdc.pcfg.GetUname(), sp.OREAD, 0, sp.MAXGETSET)
}

func (fdc *FdClient) PutFile(fname string, perm sp.Tperm, mode sp.Tmode, data []byte, off sp.Toffset, lid sp.TleaseId) (sp.Tsize, error) {
	return fdc.PathClnt.PutFile(fname, fdc.pcfg.GetUname(), mode|sp.OWRITE, perm, data, off, lid)
}

func (fdc *FdClient) NewReader(fd int, path string) *reader.Reader {
	fid, err := fdc.fds.lookup(fd)
	if err != nil {
		return nil
	}
	return fdc.PathClnt.NewReader(fid, path)
}

func (fdc *FdClient) NewWriter(fd int) *writer.Writer {
	fid, err := fdc.fds.lookup(fd)
	if err != nil {
		return nil
	}
	return fdc.PathClnt.NewWriter(fid)
}

func (fdc *FdClient) readFid(fd int, fid sp.Tfid, off sp.Toffset, cnt sp.Tsize) ([]byte, error) {
	data, err := fdc.PathClnt.ReadF(fid, off, cnt)
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
	sz, err := fdc.PathClnt.WriteF(fid, off, data, f)
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
	b, err := fdc.PathClnt.WriteRead(fid, data)
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

func (fdc *FdClient) PathLastSymlink(pn string) (path.Path, path.Path, error) {
	return fdc.PathClnt.PathLastSymlink(pn, fdc.pcfg.GetUname())
}
