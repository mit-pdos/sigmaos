package fsclnt

import (
	"sync"

	np "ulambda/ninep"
)

const (
	MAXFD = 20
)

type FdState struct {
	offset np.Toffset
	fid    np.Tfid
	mode   np.Tmode
}

type FdTable struct {
	sync.Mutex
	fds []FdState
}

func mkFdTable() *FdTable {
	fdt := &FdTable{}
	fdt.fds = make([]FdState, 0, MAXFD)
	return fdt
}

func (fdt *FdTable) findfd(nfid np.Tfid, m np.Tmode) int {
	fdt.Lock()
	defer fdt.Unlock()

	for fd, fdst := range fdt.fds {
		if fdst.fid == np.NoFid {
			fdt.fds[fd].offset = 0
			fdt.fds[fd].fid = nfid
			fdt.fds[fd].mode = m
			return fd
		}
	}
	// no free one
	fdt.fds = append(fdt.fds, FdState{0, nfid, m})
	return len(fdt.fds) - 1
}

func (fdt *FdTable) closefd(fd int) {
	fdt.Lock()
	defer fdt.Unlock()

	fdt.fds[fd].fid = np.NoFid
}

func (fdt *FdTable) lookup(fd int) (np.Tfid, *np.Err) {
	fdt.Lock()
	defer fdt.Unlock()

	if fdt.fds[fd].fid == np.NoFid {
		return np.NoFid, np.MkErr(np.TErrUnknownfid, "lookup")
	}
	return fdt.fds[fd].fid, nil
}

func (fdt *FdTable) lookupStL(fd int) (*FdState, *np.Err) {
	if fd < 0 || fd >= len(fdt.fds) {
		return nil, np.MkErr(np.TErrBadFd, fd)
	}
	if fdt.fds[fd].fid == np.NoFid {
		return nil, np.MkErr(np.TErrBadFd, fd)
	}
	return &fdt.fds[fd], nil
}

func (fdt *FdTable) lookupSt(fd int) (*FdState, *np.Err) {
	fdt.Lock()
	defer fdt.Unlock()

	fdst, err := fdt.lookupStL(fd)
	if err != nil {
		return nil, err
	}
	return fdst, nil
}

func (fdt *FdTable) setOffset(fd int, off np.Toffset) *np.Err {
	fdt.Lock()
	defer fdt.Unlock()

	fdst, err := fdt.lookupStL(fd)
	if err != nil {
		return err
	}
	fdst.offset = off
	return nil
}

// Wrote this in the CAS style, unsure if it's overkill
func (fdt *FdTable) stOffsetCAS(fd int, oldOff np.Toffset, newOff np.Toffset) (bool, *np.Err) {
	fdt.Lock()
	defer fdt.Unlock()

	if fd < 0 || fd >= len(fdt.fds) {
		return false, np.MkErr(np.TErrBadFd, fd)
	}
	if fdt.fds[fd].fid == np.NoFid {
		return false, np.MkErr(np.TErrBadFd, fd)
	}
	fdst := &fdt.fds[fd]
	if fdst.offset == oldOff {
		fdst.offset = newOff
		return true, nil
	}
	return false, nil
}
