package srv

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/userfaultfd"
)

type Req struct {
	iov      int
	nopages  int
	index    int
	addr     uint64
	realaddr uint64
}

type Range struct {
	lb uint64
	ub uint64
}

type lazyPagesConn struct {
	lps           *lazyPagesSrv
	pid           int
	imgdir        string
	pages         string // sigma path for pages file
	fd            int    // userfauldfd
	rp            func(int64, []byte) error
	pmi           *TpagemapImg
	mm            *Tmm
	iovs          *Iovs
	maxIovLen     int
	nfault        int
	npages        int
	data          []byte
	cache         [][]bool
	cachedata     []byte
	queue         []Req
	mu            sync.Mutex
	cond          *sync.Cond
	hits          int
	firstInstance bool
	histFD        int
}

func (lps *lazyPagesSrv) newLazyPagesConn(pid int, imgdir, pages string, fullpages string, histFd int) (*lazyPagesConn, error) {
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
	lpc.cache = make([][]bool, len(lpc.iovs.iovs))
	for i, iov := range lpc.iovs.iovs {
		lpc.cache[i] = make([]bool, int(iov.end-iov.start)/iov.pagesz)
	}

	if histFd != -1 {
		lpc.firstInstance = true
		lpc.histFD = histFd
	}
	f, _ := os.OpenFile(fullpages, os.O_CREATE|os.O_RDWR, 0644)
	f.Truncate(int64(pmi.nopages * pmi.pagesz))
	lpc.cachedata, err = unix.Mmap(int(f.Fd()), 0, pmi.nopages*pmi.pagesz, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		db.DPrintf(db.PROCD, "mmap err %v\n", err)
		return nil, err
	}
	lpc.queue = make([]Req, 0)
	lpc.data = make([]byte, lpc.maxIovLen)
	lpc.cond = sync.NewCond(&lpc.mu)

	db.DPrintf(db.ALWAYS, "newLazyPagesConn: img %v pages %v pid %d iovs %d npages %d maxIovLen %d", lpc.imgdir, lpc.pages, pid, lpc.iovs.len(), npages, lpc.maxIovLen)
	return lpc, nil
}

func (lpc *lazyPagesConn) handleReqs() error {
	if lpc.firstInstance {
		defer lpc.lps.CloseFd(lpc.histFD)
	}
	cnter := 0
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
			if err2 == syscall.EINTR && cnter < 10 {
				cnter += 1
				continue
			}
			return err2
		} else {
			cnter = 0
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
			time.Sleep(1 * time.Millisecond)
			_, err = syscall.Read(lpc.fd, buf)
			cnt += 1
		}
		//db.DPrintf(db.ALWAYS, "success?")
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

func (lpc *lazyPagesConn) preFetch(saved_addresses [][]uint64) {
	lpc.mu.Lock()
	mp := make([][]Req, len(lpc.iovs.iovs))
	for i, _ := range lpc.iovs.iovs {
		mp[i] = make([]Req, 0)
	}
	for _, pair := range saved_addresses {
		addr := pair[0]
		nopages := int(pair[1])
		iovno := lpc.iovs.findBinSearch(addr)

		if iovno != -1 {
			WTHRESH := 5
			iov := lpc.iovs.iovs[iovno]
			pi := lpc.pmi.findBinSearch(addr)
			index := int(addr-iov.start) / (iov.pagesz)
			iovlen := int(iov.end-iov.start) / iov.pagesz
			nopages := min(iovlen-index, nopages)
			indlow := -1
			indhigh := -1
			for i := range mp[iovno] {

				if mp[iovno][i].index <= index {
					if indlow == -1 || mp[iovno][indlow].index < mp[iovno][i].index {

						indlow = i

					}
				} else if mp[iovno][i].index > index {
					if indhigh == -1 || mp[iovno][indhigh].index > mp[iovno][i].index {
						indhigh = i
					}
				}
			}
			if indlow == -1 || mp[iovno][indlow].index+mp[iovno][indlow].nopages < index-WTHRESH {
				if indhigh == -1 || mp[iovno][indhigh].index > index+nopages+WTHRESH {
					mp[iovno] = append(mp[iovno],
						Req{iov: iovno, nopages: nopages, index: index, addr: uint64(pi * iov.pagesz), realaddr: addr})
				} else {

					mp[iovno][indhigh].addr = uint64(pi * iov.pagesz)
					mp[iovno][indhigh].nopages += mp[iovno][indhigh].index - index
					mp[iovno][indhigh].realaddr = addr
					mp[iovno][indhigh].index = index
				}
			} else {
				if mp[iovno][indlow].index+mp[iovno][indlow].nopages >= index+nopages {
					continue
				}
				if indhigh == -1 || mp[iovno][indhigh].index > index+nopages+WTHRESH {
					mp[iovno][indlow].nopages = index + nopages - mp[iovno][indlow].index

				} else {
					mp[iovno][indlow].nopages = mp[iovno][indhigh].index + mp[iovno][indhigh].nopages - mp[iovno][indlow].index

					mp[iovno] = append(mp[iovno][:indhigh], mp[iovno][indhigh+1:]...)
				}
				//

			}

		}
	}
	for iovno := range mp {
		lpc.queue = append(lpc.queue, mp[iovno]...)

	}
	db.DPrintf(db.LAZYPAGESSRV_FAULT, "QUEUE length %v saved len:%v", len(lpc.queue), len(saved_addresses))
	lpc.mu.Unlock()
	for {
		lpc.mu.Lock()
		for len(lpc.queue) == 0 {
			db.DPrintf(db.LAZYPAGESSRV_FAULT, "sleeping")
			lpc.cond.Wait() // Wait until something is appended
		}

		// Pop the first element
		lpc.hits += 1
		request := lpc.queue[0]

		lpc.queue = lpc.queue[1:]
		iov := lpc.iovs.iovs[request.iov]
		curriov := iov
		curriovno := request.iov
		iovlen := int(iov.end-iov.start) / iov.pagesz
		ind := request.index
		pref := true
		for i := 0; i < request.nopages; i++ {
			if ind >= iovlen {
				curriov = lpc.iovs.iovs[request.iov+1]
				curriovno++
				iovlen = int(curriov.end-curriov.start) / iov.pagesz

				ind = 0
			}
			if (curriov.copied[ind] || lpc.cache[curriovno][ind]) && pref {

				request.nopages--
				request.addr += uint64(iov.pagesz)
				request.index = ind + 1
				if curriovno > request.iov {
					request.iov++
					iov = lpc.iovs.iovs[request.iov]
				}
			} else {
				pref = false
			}

			ind++
		}
		lpc.mu.Unlock()
		if request.nopages == 0 {
			continue
		}
		buf := lpc.cachedata[request.addr : request.addr+uint64(request.nopages*iov.pagesz)]
		if err := lpc.rp(int64(request.addr), buf); err != nil {
			db.DFatalf("no page content for %x err: %v", request.addr, err)
		}
		lpc.mu.Lock()
		curriovno = request.iov
		iovlen = int(iov.end-iov.start) / iov.pagesz
		ind = request.index
		incremented := false
		for i := 0; i < request.nopages; i++ {
			if ind >= iovlen && !incremented {
				curriovno++
				incremented = true
				ind = 0
			}
			lpc.cache[curriovno][ind] = true
			ind++
		}
		lpc.mu.Unlock()
	}
}

func (lpc *lazyPagesConn) markAndLoad(iovno int, iov *Iov, pi int, addr0 uint64) (int, uint64) {
	THRESH := 1 + PREFETCH/2
	n := 0

	loaded := -1
	startIndex := int(addr0-iov.start) / iov.pagesz
	addr := addr0
	iovlen := int(iov.end-iov.start) / iov.pagesz
	lpc.mu.Lock()
	for ; n < THRESH && addr < iov.end; addr += uint64(iov.pagesz) {

		i := int(addr-iov.start) / iov.pagesz
		if iov.copied[i] {
			break
		}
		exists := lpc.cache[iovno][i]
		if exists && loaded == -1 {
			loaded = i - startIndex
		}
		if !exists && loaded != -1 {
			break
		}
		//iov.copied[i] = true
		n += 1
	}
	lpc.mu.Unlock()
	//is this true?
	off := int64(pi * iov.pagesz)

	if n == 0 {
		db.DPrintf(db.LAZYPAGESSRV_FAULT, "fault page: delivered %d: %d(%x) -> %v pi %d(%d,%d)", lpc.nfault, addr, addr, iov, pi, n, nPages(0, uint64(n), lpc.pmi.pagesz))
		return 0, 0
	}
	addr = addr0
	buf := lpc.data[:1]
	if loaded == -1 {
		newloaded := -1
		oldStartIndex := startIndex
		for ; startIndex > max(0, oldStartIndex-PREFETCH/2); addr -= uint64(iov.pagesz) {
			if iov.copied[startIndex-1] {
				break
			}
			exists := lpc.cache[iovno][startIndex-1]
			if exists && newloaded == -1 {
				newloaded = startIndex
			} else if !exists && newloaded != -1 {
				break
			}

			startIndex--
			n++
			off -= int64(iov.pagesz)
		}

		buf = lpc.data[:n*iov.pagesz]
		if newloaded != -1 {
			copy(buf[:(newloaded-startIndex)*iov.pagesz], lpc.cachedata[off:off+int64((newloaded-startIndex)*iov.pagesz)])
		} else {
			newloaded = startIndex
		}
		start := time.Now()
		if err := lpc.rp(off+int64((newloaded-startIndex)*iov.pagesz), buf[(newloaded-startIndex)*iov.pagesz:n*iov.pagesz]); err != nil {
			db.DFatalf("no page content for %x err: %v", addr, err)
		}
		duration := time.Since(start)
		db.DPrintf(db.LAZYPAGESSRV_FAULT, "rp time taken %v pages: %v", duration.Microseconds(), n-(newloaded-startIndex))
	} else {
		if loaded > 0 {
			newloaded := -1
			oldStartIndex := startIndex
			for ; startIndex > max(0, oldStartIndex-PREFETCH/2); addr -= uint64(iov.pagesz) {
				if iov.copied[startIndex-1] {
					break
				}
				exists := lpc.cache[iovno][startIndex-1]
				if exists && newloaded == -1 {
					newloaded = startIndex
				} else if !exists && newloaded != -1 {
					break
				}

				startIndex--
				n++
				loaded++
				off -= int64(iov.pagesz)
			}
			buf = lpc.data[:n*iov.pagesz]
			if newloaded != -1 {
				copy(buf[:(newloaded-startIndex)*iov.pagesz], lpc.cachedata[off:off+int64((newloaded-startIndex)*iov.pagesz)])
			} else {
				newloaded = startIndex
			}
			if err := lpc.rp(off+int64((newloaded-startIndex)*iov.pagesz), buf[(newloaded-startIndex)*iov.pagesz:loaded*iov.pagesz]); err != nil {
				db.DFatalf("no page content for %x err: %v", addr, err)
			}
		} else {
			oldStartIndex := startIndex
			for ; startIndex > max(0, oldStartIndex-PREFETCH/2); addr -= uint64(iov.pagesz) {
				exists := lpc.cache[iovno][startIndex-1]

				if !exists || iov.copied[startIndex-1] {
					break
				}
				startIndex--
				n++
				off -= int64(iov.pagesz)
			}
			buf = lpc.data[:n*iov.pagesz]
		}
		start := time.Now()
		copy(buf[loaded*iov.pagesz:n*iov.pagesz], lpc.cachedata[int64(loaded*iov.pagesz)+off:off+int64(n*iov.pagesz)])
		duration := time.Since(start)
		db.DPrintf(db.LAZYPAGESSRV_FAULT, "skip copy time taken %v pages: %v ", duration.Microseconds(), n-loaded)
	}
	if lpc.firstInstance {
		db.DPrintf(db.LAZYPAGESSRV_FAULT, "hist: %v writing %s", lpc.histFD, fmt.Sprintf("0x%x %v\n", addr, n))
		_, err := lpc.lps.Write(lpc.histFD, []byte(fmt.Sprintf("0x%x %d\n", addr, n)))
		if err != nil {
			db.DPrintf(db.ERROR, "error hist: %v", err)
		}
	} else {
		db.DPrintf(db.LAZYPAGESSRV_FAULT, "iov %v faulting copy %x->(%x), bounds: %v-%v, iovlen %v, len: %v, preloaded? %v", iovno, addr, addr, startIndex, startIndex+n, iovlen, n, loaded)

	}
	return n * iov.pagesz, addr
}
func (lpc *lazyPagesConn) pageFault(addr uint64) error {
	iovno := lpc.iovs.findBinSearch(addr)
	var iov *Iov = nil
	if iovno != -1 {
		iov = lpc.iovs.iovs[iovno]
	}
	if iov == nil {
		lowerbound := addr
		upperbound := addr + uint64(lpc.pmi.pagesz)

		db.DPrintf(db.LAZYPAGESSRV_FAULT, "page fault: zero %d: no iov for %x, lb: %x ub: %x", lpc.nfault, addr, lowerbound, upperbound)
		err := zeroPage(lpc.fd, addr, lpc.lps.pagesz)
		cnt := 0
		for err != nil {

			db.DPrintf(db.ERROR, "zeroPage err %v try: %v", err, cnt)
			if err == syscall.ESRCH {
				return nil
			}
			if err == syscall.EEXIST {
				return nil
			}
			if cnt > 10 || (err != syscall.EAGAIN && err != syscall.ENOENT) {

				return err
			}
			if err == syscall.EAGAIN {
				return nil
			}
			time.Sleep(1 * time.Millisecond)

			err = zeroPage(lpc.fd, addr, lpc.lps.pagesz)
			cnt += 1
		}
	} else {
		pi := lpc.pmi.findBinSearch(addr)
		addr0 := addr
		if pi == -1 {
			db.DFatalf("no page for %x", addr)
		}
		n, addr := lpc.markAndLoad(iovno, iov, pi, addr)
		if n == 0 {
			db.DPrintf(db.LAZYPAGESSRV_FAULT, "fault page: delivered %d: %d(%x) -> %v pi %d(%d,%d)", lpc.nfault, addr, addr, iovno, pi, n, nPages(0, uint64(n), lpc.pmi.pagesz))
			return nil
		}
		lpc.npages += 1
		buf := lpc.data[0:n]
		db.DPrintf(db.LAZYPAGESSRV_FAULT, "page fault: copy %d: %x old %d(%x) -> %v pi %d(%d,%d,%d) npages(%d)", lpc.nfault, addr0, addr, addr, iovno, pi, n, nPages(0, uint64(n), lpc.pmi.pagesz), len(buf), lpc.npages)

		err := copyPages(lpc.fd, addr, buf)
		cnt := 0
		for err != nil {
			db.DPrintf(db.ERROR, "copyPages err %v try: %v busy:%v ESRCH: %v", err, cnt, err == syscall.EBUSY, err == syscall.ESRCH)
			if err == syscall.ESRCH {
				return nil
			}
			if cnt > 10 || (err != syscall.EBUSY && err != syscall.ENOENT && err != syscall.EAGAIN) {

				return err
			}
			if err == syscall.EAGAIN {
				return nil
			}

			time.Sleep(5 * time.Millisecond)
			err = copyPages(lpc.fd, addr, buf)
			cnt += 1
		}
		sindex := (addr - iov.start) / uint64(iov.pagesz)
		for index := sindex; index < sindex+uint64(n/iov.pagesz); index++ {
			iov.copied[index] = true
		}
	}

	return nil
}

func zeroLen(fd int, addr uint64, len uint64) error {
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
