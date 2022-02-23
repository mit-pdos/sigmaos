package fdclnt

import (
	"fmt"

	"ulambda/fidclnt"
	np "ulambda/ninep"
	"ulambda/reader"
	"ulambda/writer"
)

//
// Procs interact with FidClient through FdClient using Unix-like file
// descriptor interface. A hypothetical kernel could multiplex
// multiple procs over one FidClient, which allows a shared TCP
// connection to a server. A kernel could also use fds to share file
// descriptors state (e.g., offset) between parent and child.  Since
// we have no kernel implementing procs, these use cases are
// speculative. Our use case is one FdClient per proc, and each
// FdClient with their own FidClient (i.e., no sharing).
//

type FdClient struct {
	*fidclnt.FidClient
	fds   *FdTable
	uname string // the principal associated with this FdClient
}

func MakeFdClient(fsc *fidclnt.FidClient, uname string) *FdClient {
	fdc := &FdClient{}
	if fsc == nil {
		fdc.FidClient = fidclnt.MakeFidClient()
	} else {
		fdc.FidClient = fsc
	}
	fdc.fds = mkFdTable()
	fdc.uname = uname
	return fdc
}

func (fdc *FdClient) String() string {
	str := fmt.Sprintf("Table:\n")
	str += fmt.Sprintf("fds %v\n", fdc.fds)
	str += fmt.Sprintf("fsc %v\n", fdc.FidClient)
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
	err := fdc.FidClient.Close(fid)
	if err != nil {
		return err
	}
	return nil
}

func (fdc *FdClient) Create(path string, perm np.Tperm, mode np.Tmode) (int, error) {
	fid, err := fdc.FidClient.Create(path, perm, mode)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) OpenWatch(path string, mode np.Tmode, w fidclnt.Watch) (int, error) {
	fid, err := fdc.FidClient.OpenWatch(path, mode, w)
	if err != nil {
		return -1, err
	}
	fd := fdc.fds.allocFd(fid, mode)
	return fd, nil
}

func (fdc *FdClient) Open(path string, mode np.Tmode) (int, error) {
	return fdc.OpenWatch(path, mode, nil)
}

func (fdc *FdClient) MakeReader(fd int, chunksz np.Tsize) (*reader.Reader, error) {
	fid, error := fdc.fds.lookup(fd)
	if error != nil {
		return nil, error
	}
	return reader.MakeReader(fdc.FidClient, fid, chunksz)
}

func (fdc *FdClient) MakeWriter(fd int, chunksz np.Tsize) (*writer.Writer, error) {
	fid, error := fdc.fds.lookup(fd)
	if error != nil {
		return nil, error
	}
	return writer.MakeWriter(fdc.FidClient, fid, chunksz)
}

func (fdc *FdClient) Read(fd int, cnt np.Tsize) ([]byte, error) {
	fid, off, error := fdc.fds.lookupOff(fd)
	if error != nil {
		return nil, error
	}
	data, err := fdc.FidClient.Read(fid, off, cnt)
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
	sz, err := fdc.FidClient.Write(fid, off, data)
	if err != nil {
		return 0, err
	}
	fdc.fds.incOff(fd, np.Toffset(sz))
	return sz, nil
}
