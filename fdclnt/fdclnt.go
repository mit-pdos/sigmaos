package fdclnt

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	np "sigmaos/ninep"
	"sigmaos/pathclnt"
	"sigmaos/reader"
	"sigmaos/writer"
)

//
// Procs interact with servers using Unix-like file descriptor
// interface and pathnames. The file descriptor operation are here,
// while pathname operations are inherited from PathClnt.
//
// A hypothetical kernel could multiplex multiple procs over one
// FidClnt, which allows a shared TCP connection to a server. A kernel
// could also use fds to share file descriptors state (e.g., offset)
// between parent and child.  Since we have no kernel implementing
// procs, these use cases are speculative. Our use case is one
// FdClient per proc, and each FdClient with their own FidClnt (i.e.,
// no sharing).
//

type FdClient struct {
	*pathclnt.PathClnt
	fds   *FdTable
	uname string // the principal associated with this FdClient
}

func MakeFdClient(fsc *fidclnt.FidClnt, uname string, sz np.Tsize) *FdClient {
	fdc := &FdClient{}
	fdc.PathClnt = pathclnt.MakePathClnt(fsc, sz)
	fdc.fds = mkFdTable()
	fdc.uname = uname
	return fdc
}

func (fdc *FdClient) String() string {
	str := fmt.Sprintf("Table:\n")
	str += fmt.Sprintf("fds %v\n", fdc.fds)
	str += fmt.Sprintf("fsc %v\n", fdc.PathClnt)
	return str
}

func (fdc *FdClient) Uname() string {
	return fdc.uname
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

func (fdc *FdClient) Qid(fd int) (np.Tqid, error) {
	fid, error := fdc.fds.lookup(fd)
	if error != nil {
		return np.Tqid{}, error
	}
	return fdc.PathClnt.Qid(fid), nil
}

func (fdc *FdClient) Create(path string, perm np.Tperm, mode np.Tmode) (int, error) {
	fid, err := fdc.PathClnt.Create(path, perm, mode)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) OpenWatch(path string, mode np.Tmode, w pathclnt.Watch) (int, error) {
	fid, err := fdc.PathClnt.OpenWatch(path, mode, w)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) Open(path string, mode np.Tmode) (int, error) {
	return fdc.OpenWatch(path, mode, nil)
}

func (fdc *FdClient) CreateOpen(path string, perm np.Tperm, mode np.Tmode) (int, error) {
	fd, err := fdc.Create(path, perm, mode)
	if err != nil && !np.IsErrExists(err) {
		db.DPrintf("FDCLNT_ERR", "Create %v err %v", path, err)
		return -1, err
	}
	if err != nil {
		fd, err = fdc.Open(path, mode)
		if err != nil {
			db.DPrintf("FDCLNT_ERR", "Open %v err %v", path, err)
			return -1, err
		}
	}
	return fd, nil
}

func (fdc *FdClient) MakeReader(fd int, path string, chunksz np.Tsize) *reader.Reader {
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

func (fdc *FdClient) readFid(fd int, fid np.Tfid, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	data, err := fdc.PathClnt.ReadV(fid, off, cnt, v)
	if err != nil {
		return nil, err
	}
	fdc.fds.incOff(fd, np.Toffset(len(data)))
	return data, nil
}

func (fdc *FdClient) ReadV(fd int, cnt np.Tsize) ([]byte, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return nil, error
	}
	qid := fdc.PathClnt.Qid(fid)
	return fdc.readFid(fd, fid, off, cnt, qid.Version)
}

func (fdc *FdClient) Read(fd int, cnt np.Tsize) ([]byte, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return nil, error
	}
	return fdc.readFid(fd, fid, off, cnt, np.NoV)
}

func (fdc *FdClient) writeFid(fd int, fid np.Tfid, off np.Toffset, data []byte, v np.TQversion) (np.Tsize, error) {
	sz, err := fdc.PathClnt.WriteV(fid, off, data, v)
	if err != nil {
		return 0, err
	}
	fdc.fds.incOff(fd, np.Toffset(sz))
	return sz, nil
}

func (fdc *FdClient) WriteV(fd int, data []byte) (np.Tsize, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return 0, error
	}
	qid := fdc.PathClnt.Qid(fid)
	return fdc.writeFid(fd, fid, off, data, qid.Version)
}

func (fdc *FdClient) Write(fd int, data []byte) (np.Tsize, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return 0, error
	}
	return fdc.writeFid(fd, fid, off, data, np.NoV)
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

func (fdc *FdClient) Seek(fd int, off np.Toffset) error {
	err := fdc.fds.setOffset(fd, off)
	if err != nil {
		return err
	}
	return nil
}
