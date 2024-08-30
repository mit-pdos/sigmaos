package lazypagessrv

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/userfaultfd"
)

type lazyPagesConn struct {
	lps       *lazyPagesSrv
	pid       int
	fd        int
	rp        func(int64, []byte) error
	pmi       *TpagemapImg
	mm        *Tmm
	iovs      *Iovs
	maxIovLen int
	nfault    int
	pages     []byte
}

func (lps *lazyPagesSrv) newLazyPagesConn(fd int, rp func(int64, []byte) error, pid int) (*lazyPagesConn, error) {
	lpc := &lazyPagesConn{lps: lps, fd: fd, rp: rp, pid: int(pid)}
	pmi, err := newTpagemapImg(lps.imgdir, "1")
	if err != nil {
		return nil, err
	}
	lpc.pmi = pmi
	mm, err := newTmm(lps.imgdir, "1")
	if err != nil {
		return nil, err
	}
	lpc.mm = mm
	npages := 0
	lpc.iovs, npages, lpc.maxIovLen = mm.collectIovs(pmi)
	lpc.pages = make([]byte, lpc.maxIovLen)
	db.DPrintf(db.ALWAYS, "lazypages: img %v pages %v pid %d fd %d iovs %d npages %d maxIovLen %d", lps.imgdir, lps.pages, pid, fd, lpc.iovs.len(), npages, lpc.maxIovLen)
	return lpc, nil
}

func (lpc *lazyPagesConn) handleReqs() error {
	for {
		if _, err := unix.Poll(
			[]unix.PollFd{{
				Fd:     int32(lpc.fd),
				Events: unix.POLLIN,
			}},
			-1,
		); err != nil {
			return err
		}
		buf := make([]byte, unsafe.Sizeof(userfaultfd.UffdMsg{}))
		if _, err := syscall.Read(lpc.fd, buf); err != nil {
			return err
		}
		msg := (*(*userfaultfd.UffdMsg)(unsafe.Pointer(&buf[0])))
		switch userfaultfd.GetMsgEvent(&msg) {
		case userfaultfd.UFFD_EVENT_PAGEFAULT:
			arg := userfaultfd.GetMsgArg(&msg)
			pagefault := (*(*userfaultfd.UffdPagefault)(unsafe.Pointer(&arg[0])))
			addr := uint64(userfaultfd.GetPagefaultAddress(&pagefault))
			mask := uint64(^(lpc.lps.pagesz - 1))
			if err := lpc.pageFault(addr & mask); err != nil {
				return err
			}
		case userfaultfd.UFFD_EVENT_FORK:
			db.DPrintf(db.ERROR, "Fork event %v", userfaultfd.GetMsgEvent(&msg))
		case userfaultfd.UFFD_EVENT_REMAP:
			db.DPrintf(db.ERROR, "Remap event %v", userfaultfd.GetMsgEvent(&msg))
		case userfaultfd.UFFD_EVENT_REMOVE:
			db.DPrintf(db.ERROR, "Remove event %v", userfaultfd.GetMsgEvent(&msg))
		case userfaultfd.UFFD_EVENT_UNMAP:
			db.DPrintf(db.ERROR, "Unmap event %v", userfaultfd.GetMsgEvent(&msg))
		default:
			return fmt.Errorf("Unknown event %x", userfaultfd.GetMsgEvent(&msg))
		}
	}
}

func (lpc *lazyPagesConn) pageFault(addr uint64) error {
	iov := lpc.iovs.find(addr)
	lpc.nfault += 1
	if iov == nil {
		db.DPrintf(db.LAZYPAGESSRV, "page fault: zero %d: no iov for %x", lpc.nfault, addr)
		zeroPage(lpc.fd, addr, lpc.lps.pagesz)
	} else {
		pi := lpc.pmi.find(addr)
		if pi == -1 {
			db.DFatalf("no page for %x", addr)
		}
		n := iov.markFetchLen(addr)
		if n == 0 {
			db.DPrintf(db.LAZYPAGESSRV, "fault page: delivered %d: %d(%x) -> %v pi %d(%d,%d)", lpc.nfault, addr, addr, iov, pi, n, nPages(0, uint64(n), lpc.pmi.pagesz))
			return nil
		}
		buf := lpc.pages[0:n]
		db.DPrintf(db.LAZYPAGESSRV, "page fault: copy %d: %d(%x) -> %v pi %d(%d,%d,%d)", lpc.nfault, addr, addr, iov, pi, n, nPages(0, uint64(n), lpc.pmi.pagesz), len(buf))
		off := int64(pi * lpc.pmi.pagesz)
		if err := lpc.rp(off, buf); err != nil {
			db.DFatalf("no page content for %x", addr)
		}
		copyPages(lpc.fd, addr, buf)
	}
	return nil
}

func zeroPage(fd int, addr uint64, pagesz int) {
	len := uint64(pagesz)
	zero := userfaultfd.NewUffdioZeroPage(
		userfaultfd.CULong(addr),
		userfaultfd.CULong(len),
		0,
	)
	if _, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		userfaultfd.UFFDIO_ZEROPAGE,
		uintptr(unsafe.Pointer(&zero)),
	); errno != 0 {
		db.DPrintf(db.ERROR, "SYS_IOCTL err %v", errno)
	}
}

func copyPages(fd int, addr uint64, pages []byte) {
	len := len(pages)
	cpy := userfaultfd.NewUffdioCopy(
		pages,
		userfaultfd.CULong(addr),
		userfaultfd.CULong(len),
		0,
		0,
	)
	if _, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		userfaultfd.UFFDIO_COPY,
		uintptr(unsafe.Pointer(&cpy)),
	); errno != 0 {
		db.DPrintf(db.ERROR, "SYS_IOCTL %d(%x) %d err %v", addr, addr, len, errno)
	}
}

func readBytesSigma(sc *sigmaclnt.SigmaClnt, fd int, off int64, buf []byte) error {
	if n, err := sc.Pread(fd, buf, sp.Toffset(off)); err != nil {
		return err
	} else if int(n) != len(buf) {
		db.DPrintf(db.LAZYPAGESSRV, "read %d n = %d", len(buf), n)
	}
	return nil
}
