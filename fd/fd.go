package fd

import (
	"sync"

	np "ulambda/ninep"
)

const (
	MAXFD = 20
)

type FdState struct {
	offset np.Toffset // unused for now
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

func (fdt *FdTable) allocFd(nfid np.Tfid, m np.Tmode) int {
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

// Caller must have locked fdt
func (fdt *FdTable) lookupL(fd int) (*FdState, *np.Err) {
	if fd < 0 || fd >= len(fdt.fds) {
		return nil, np.MkErr(np.TErrBadFd, fd)
	}
	if fdt.fds[fd].fid == np.NoFid {
		return nil, np.MkErr(np.TErrBadFd, fd)
	}
	return &fdt.fds[fd], nil
}

func (fdt *FdTable) lookup(fd int) (np.Tfid, *np.Err) {
	fdt.Lock()
	defer fdt.Unlock()

	st, err := fdt.lookupL(fd)
	if err != nil {
		return np.NoFid, err
	}
	return st.fid, nil
}

func (fdt *FdTable) lookupOff(fd int) (np.Tfid, np.Toffset, *np.Err) {
	fdt.Lock()
	defer fdt.Unlock()

	st, err := fdt.lookupL(fd)
	if err != nil {
		return np.NoFid, 0, err
	}
	return st.fid, st.offset, nil
}

func (fdt *FdTable) setOffset(fd int, off np.Toffset) *np.Err {
	fdt.Lock()
	defer fdt.Unlock()

	st, err := fdt.lookupL(fd)
	if err != nil {
		return err
	}
	st.offset = off
	return nil
}

func (fdt *FdTable) incOff(fd int, off np.Toffset) *np.Err {
	fdt.Lock()
	defer fdt.Unlock()

	st, err := fdt.lookupL(fd)
	if err != nil {
		return err
	}
	st.offset += off
	return nil
}
