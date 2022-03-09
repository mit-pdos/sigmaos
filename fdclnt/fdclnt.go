package fdclnt

import (
	"fmt"

	"ulambda/fidclnt"
	np "ulambda/ninep"
	"ulambda/pathclnt"
	"ulambda/reader"
	"ulambda/writer"
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

func (fdc *FdClient) MakeReader(fd int, path string, chunksz np.Tsize) *reader.Reader {
	fid, err := fdc.fds.lookup(fd)
	if err != nil {
		return nil
	}
	return fdc.PathClnt.MakeReader(fid, path, chunksz)
}

func (fdc *FdClient) MakeWriter(fd int, chunksz np.Tsize) *writer.Writer {
	fid, err := fdc.fds.lookup(fd)
	if err != nil {
		return nil
	}
	return fdc.PathClnt.MakeWriter(fid, chunksz)
}

func (fdc *FdClient) Read(fd int, cnt np.Tsize) ([]byte, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return nil, error
	}
	data, err := fdc.PathClnt.Read(fid, off, cnt)
	if err != nil {
		return nil, err
	}
	fdc.fds.incOff(fd, np.Toffset(len(data)))
	return data, nil
}

func (fdc *FdClient) Write(fd int, data []byte) (np.Tsize, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return 0, error
	}
	sz, err := fdc.PathClnt.Write(fid, off, data)
	if err != nil {
		return 0, err
	}
	fdc.fds.incOff(fd, np.Toffset(sz))
	return sz, nil
}
