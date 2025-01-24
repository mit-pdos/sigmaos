package srv

import (
	"fmt"
	"strconv"
	"syscall"
	"time"
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
	imgdir    string
	pages     string // sigma path for pages file
	fd        int    // userfauldfd
	rp        func(int64, []byte) error
	pmi       *TpagemapImg
	mm        *Tmm
	iovs      *Iovs
	maxIovLen int
	nfault    int
	data      []byte
}

func (lps *lazyPagesSrv) newLazyPagesConn(pid int, imgdir, pages string) (*lazyPagesConn, error) {
	lpc := &lazyPagesConn{lps: lps, pid: pid, imgdir: imgdir, pages: pages}
	pidstr := strconv.Itoa(pid)
	pmi, err := newTpagemapImg(lpc.imgdir, pidstr)
	if err != nil {
		return nil, err
	}
	lpc.pmi = pmi
	mm, err := newTmm(lpc.imgdir, pidstr)
	if err != nil {
		return nil, err
	}
	lpc.mm = mm
	npages := 0
	lpc.iovs, npages, lpc.maxIovLen = mm.collectIovs(pmi)
	lpc.data = make([]byte, lpc.maxIovLen)
	db.DPrintf(db.ALWAYS, "newLazyPagesConn: img %v pages %v pid %d iovs %d npages %d maxIovLen %d", lpc.imgdir, lpc.pages, pid, lpc.iovs.len(), npages, lpc.maxIovLen)
	return lpc, nil
}

func (lpc *lazyPagesConn) handleReqs() error {
	for {
		n, err2 := unix.Poll(
			[]unix.PollFd{{
				Fd:     int32(lpc.fd),
				Events: unix.POLLIN,
			}},
			-1,
		)
		if err2 != nil {
			db.DPrintf(db.ERROR, "Poll err %v", err2)
			return err2
		}
		// if n, err := unix.Poll(
		// 	[]unix.PollFd{{
		// 		Fd:     int32(lpc.fd),
		// 		Events: unix.POLLIN,
		// 	}},
		// 	-1,
		// ); err != nil {
		// 	db.DPrintf(db.ERROR, "Poll err %v", err)
		// 	return err
		// }
		buf := make([]byte, unsafe.Sizeof(userfaultfd.UffdMsg{}))
		n2, err := syscall.Read(lpc.fd, buf)
		cnt := 0

		for err != nil {
			db.DPrintf(db.ERROR, "Read err %v try: %v n: %v n2: %v", err, cnt, n, n2)
			if err == syscall.ENOENT {

				return nil
			}
			if cnt > 10 || err != syscall.EAGAIN {

				return err
			}
			time.Sleep(100 * time.Millisecond)
			_, err = syscall.Read(lpc.fd, buf)
			cnt += 1
		}
		db.DPrintf(db.ALWAYS, "success?")
		msg := (*(*userfaultfd.UffdMsg)(unsafe.Pointer(&buf[0])))
		switch userfaultfd.GetMsgEvent(&msg) {
		case userfaultfd.UFFD_EVENT_PAGEFAULT:
			arg := userfaultfd.GetMsgArg(&msg)
			pagefault := (*(*userfaultfd.UffdPagefault)(unsafe.Pointer(&arg[0])))
			addr := uint64(userfaultfd.GetPagefaultAddress(&pagefault))
			mask := uint64(^(lpc.lps.pagesz - 1))
			if err := lpc.pageFault(addr & mask); err != nil {
				db.DPrintf(db.ERROR, "Page Fault err %v", err)
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
		db.DPrintf(db.LAZYPAGESSRV_FAULT, "page fault: zero %d: no iov for %x", lpc.nfault, addr)
		err := zeroPage(lpc.fd, addr, lpc.lps.pagesz)
		cnt := 0
		for err != nil {
			db.DPrintf(db.ERROR, "zeroPage err %v try: %v", err, cnt)
			if cnt > 10 || err != syscall.EAGAIN {

				return err
			}
			if err == syscall.EAGAIN {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
			err = zeroPage(lpc.fd, addr, lpc.lps.pagesz)
			cnt += 1
		}
	} else {
		pi := lpc.pmi.find(addr)
		if pi == -1 {
			db.DFatalf("no page for %x", addr)
		}
		n := iov.markFetchLen(addr)
		if n == 0 {
			db.DPrintf(db.LAZYPAGESSRV_FAULT, "fault page: delivered %d: %d(%x) -> %v pi %d(%d,%d)", lpc.nfault, addr, addr, iov, pi, n, nPages(0, uint64(n), lpc.pmi.pagesz))
			return nil
		}
		buf := lpc.data[0:n]
		db.DPrintf(db.LAZYPAGESSRV_FAULT, "page fault: copy %d: %d(%x) -> %v pi %d(%d,%d,%d)", lpc.nfault, addr, addr, iov, pi, n, nPages(0, uint64(n), lpc.pmi.pagesz), len(buf))
		off := int64(pi * lpc.pmi.pagesz)
		if err := lpc.rp(off, buf); err != nil {
			db.DFatalf("no page content for %x", addr)
		}

		err := copyPages(lpc.fd, addr, buf)
		cnt := 0
		for err != nil {
			db.DPrintf(db.ERROR, "copyPages err %v try: %v int: %v", err, cnt, unsafe.Sizeof(int(0)))
			if err == syscall.ENOENT {

				if n <= lpc.pmi.pagesz {
					return err
				}
				n -= lpc.pmi.pagesz
				iov.unmarkFetchLen(addr+uint64(n), 1)
				buf = lpc.data[0:n]
				off := int64(pi * lpc.pmi.pagesz)
				if err := lpc.rp(off, buf); err != nil {
					db.DFatalf("no page content for %x", addr)
				}
				db.DPrintf(db.LAZYPAGESSRV_FAULT, "page fault: copy %d: %d(%x) -> %v pi %d(%d,%d,%d)", lpc.nfault, addr, addr, iov, pi, n, nPages(0, uint64(n), lpc.pmi.pagesz), len(buf))
				err = copyPages(lpc.fd, addr, buf)
				//		iov.unmarkFetchLen(addr, n)

				continue
			}
			if cnt > 10 || (err != syscall.EAGAIN && err != syscall.ENOENT) {

				return err
			}
			if err == syscall.EAGAIN {
				return nil
			}

			time.Sleep(100 * time.Millisecond)
			err = copyPages(lpc.fd, addr, buf)
			cnt += 1
		}
		// if err != nil {
		// 	db.DPrintf(db.ERROR, "SYS_IOCTL1 err %v", err)
		// 	// 	time.Sleep(1000 * time.Millisecond)
		// 	// 	copyPages(lpc.fd, addr, buf)

		//}
	}
	return nil
}

func zeroPage(fd int, addr uint64, pagesz int) error {
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
		return errno
	}
	return nil
}

func copyPages(fd int, addr uint64, pages []byte) error {
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
		db.DPrintf(db.ERROR, "SYS_IOCTL %d(%x) %d err %v copy %v", addr, addr, len, errno, cpy)
		return errno
	}
	db.DPrintf(db.LAZYPAGESSRV_FAULT, "SYS_IOCTL %d(%x) %d copy %v", addr, addr, len, cpy)
	return nil
}

func readBytesSigma(sc *sigmaclnt.SigmaClnt, fd int, off int64, buf []byte) error {
	if n, err := sc.Pread(fd, buf, sp.Toffset(off)); err != nil {
		return err
	} else if int(n) != len(buf) {
		db.DPrintf(db.LAZYPAGESSRV, "read %d n = %d", len(buf), n)
	}
	return nil
}
