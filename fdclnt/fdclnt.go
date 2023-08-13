package fdclnt

import (
	"fmt"

	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/path"
	"sigmaos/pathclnt"
	"sigmaos/reader"
	"sigmaos/serr"
	"sigmaos/sessp"
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
	scfg *config.SigmaConfig
	*pathclnt.PathClnt
	fds *FdTable
}

func MakeFdClient(scfg *config.SigmaConfig, fsc *fidclnt.FidClnt, sz sessp.Tsize) *FdClient {
	fdc := &FdClient{scfg: scfg}
	fdc.PathClnt = pathclnt.MakePathClnt(scfg, fsc, sz)
	fdc.fds = mkFdTable()
	return fdc
}

func (fdc *FdClient) String() string {
	str := fmt.Sprintf("Table:\n")
	str += fmt.Sprintf("fds %v\n", fdc.fds)
	str += fmt.Sprintf("fsc %v\n", fdc.PathClnt)
	return str
}

func (fdc *FdClient) Uname() sp.Tuname {
	return fdc.scfg.Uname
}

func (fdc *FdClient) Close(fd int) error {
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
	return fdc.PathClnt.Stat(name, fdc.scfg.Uname)
}

func (fdc *FdClient) Create(path string, perm sp.Tperm, mode sp.Tmode) (int, error) {
	fid, err := fdc.PathClnt.Create(path, fdc.scfg.Uname, perm, mode, sp.NoLeaseId)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) CreateEphemeral(path string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId) (int, error) {
	fid, err := fdc.PathClnt.Create(path, fdc.scfg.Uname, perm|sp.DMTMP, mode, lid)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) OpenWatch(path string, mode sp.Tmode, w pathclnt.Watch) (int, error) {
	fid, err := fdc.PathClnt.OpenWatch(path, fdc.scfg.Uname, mode, w)
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
	return fdc.PathClnt.SetRemoveWatch(pn, fdc.scfg.Uname, w)
}

func (fdc *FdClient) Rename(old, new string) error {
	return fdc.PathClnt.Rename(old, new, fdc.scfg.Uname)
}

func (fdc *FdClient) Remove(pn string) error {
	return fdc.PathClnt.Remove(pn, fdc.scfg.Uname)
}

func (fdc *FdClient) GetFile(fname string) ([]byte, error) {
	return fdc.PathClnt.GetFile(fname, fdc.scfg.Uname, sp.OREAD, 0, sp.MAXGETSET)
}

func (fdc *FdClient) PutFile(fname string, perm sp.Tperm, mode sp.Tmode, data []byte, off sp.Toffset, lid sp.TleaseId) (sessp.Tsize, error) {
	return fdc.PathClnt.PutFile(fname, fdc.scfg.Uname, mode|sp.OWRITE, perm, data, off, lid)
}

func (fdc *FdClient) MakeReader(fd int, path string, chunksz sessp.Tsize) *reader.Reader {
	fid, err := fdc.fds.lookup(fd)
	if err != nil {
		return nil
	}
	return fdc.PathClnt.MakeReader(fid, path, chunksz)
}

func (fdc *FdClient) MakeWriter(fd int) *writer.Writer {
	fid, err := fdc.fds.lookup(fd)
	if err != nil {
		return nil
	}
	return fdc.PathClnt.MakeWriter(fid)
}

func (fdc *FdClient) readFid(fd int, fid sp.Tfid, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, error) {
	data, err := fdc.PathClnt.ReadV(fid, off, cnt, v)
	if err != nil {
		return nil, err
	}
	fdc.fds.incOff(fd, sp.Toffset(len(data)))
	return data, nil
}

func (fdc *FdClient) ReadV(fd int, cnt sessp.Tsize) ([]byte, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return nil, error
	}
	qid := fdc.PathClnt.Qid(fid)
	return fdc.readFid(fd, fid, off, cnt, qid.Tversion())
}

func (fdc *FdClient) Read(fd int, cnt sessp.Tsize) ([]byte, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return nil, error
	}
	return fdc.readFid(fd, fid, off, cnt, sp.NoV)
}

func (fdc *FdClient) writeFid(fd int, fid sp.Tfid, off sp.Toffset, data []byte, v sp.TQversion) (sessp.Tsize, error) {
	sz, err := fdc.PathClnt.WriteV(fid, off, data, v)
	if err != nil {
		return 0, err
	}
	fdc.fds.incOff(fd, sp.Toffset(sz))
	return sz, nil
}

func (fdc *FdClient) WriteV(fd int, data []byte) (sessp.Tsize, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return 0, error
	}
	qid := fdc.PathClnt.Qid(fid)
	return fdc.writeFid(fd, fid, off, data, qid.Tversion())
}

func (fdc *FdClient) Write(fd int, data []byte) (sessp.Tsize, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return 0, error
	}
	return fdc.writeFid(fd, fid, off, data, sp.NoV)
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
	return fdc.PathClnt.PathLastSymlink(pn, fdc.scfg.Uname)
}
