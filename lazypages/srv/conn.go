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
	npages    int
	data      []byte
	//hist      []Range
	//cache     map[int][]bool
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
	//	for _, vma := range mm.Vmas {
	// 	db.DPrintf(db.ALWAYS, "vma start: %x end: %x", *vma.Start, *vma.End)
	// }
	lpc.mm = mm
	npages := 0
	//lpc.hist = make([]Range, 0)
	lpc.iovs, npages, lpc.maxIovLen = mm.collectIovs(pmi)
	//lpc.cache = make(map[int][]bool)
	lpc.cache = make([][]bool, len(lpc.iovs.iovs))
	for i, iov := range lpc.iovs.iovs {
		lpc.cache[i] = make([]bool, int(iov.end-iov.start)/iov.pagesz)
	}

	if histFd != -1 {
		lpc.firstInstance = true
		lpc.histFD = histFd
	}
	// if err := lps.DownloadFile(pages, fullpages); err != nil {
	// 	db.DPrintf(db.PROCD, "DownloadFile pages err %v\n", err)
	// 	return nil, err
	// }
	//f, _ := os.OpenFile(fullpages, os.O_RDWR, 0644)
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

func reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
func (lpc *lazyPagesConn) preFetch(saved_addresses [][]uint64) {
	//saved_addresses = make([][]uint64, 0)
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
					//		db.DPrintf(db.LAZYPAGESSRV_FAULT, "new : %v+%v", index, nopages)
					mp[iovno] = append(mp[iovno],
						Req{iov: iovno, nopages: nopages, index: index, addr: uint64(pi * iov.pagesz), realaddr: addr})
				} else {
					//		db.DPrintf(db.LAZYPAGESSRV_FAULT, "extend : %v -> %v+%v", mp[iovno][indhigh].index, index, mp[iovno][indhigh].nopages)

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
					//		db.DPrintf(db.LAZYPAGESSRV_FAULT, "extend : %v -> %v+%v", mp[iovno][indlow].index, index, nopages)

				} else {
					mp[iovno][indlow].nopages = mp[iovno][indhigh].index + mp[iovno][indhigh].nopages - mp[iovno][indlow].index
					//		db.DPrintf(db.LAZYPAGESSRV_FAULT, "replace : %v -> %v+%v", mp[iovno][indhigh].index, mp[iovno][indlow].index, mp[iovno][indlow].nopages)

					mp[iovno] = append(mp[iovno][:indhigh], mp[iovno][indhigh+1:]...)
				}
				//

			}

		}
	}
	for iovno := range mp {
		lpc.queue = append(lpc.queue, mp[iovno]...)

	}
	// mp := make(map[int]int)
	// for _, pair := range saved_addresses {
	// 	addr := pair[0]
	// 	nopages := int(pair[1])
	// 	iovno := lpc.iovs.findBinSearch(addr)

	// 	if iovno != -1 {

	// 		iov := lpc.iovs.iovs[iovno]
	// 		pi := lpc.pmi.findBinSearch(addr)
	// 		index := int(addr-iov.start) / (iov.pagesz)
	// 		iovlen := int(iov.end-iov.start) / iov.pagesz
	// 		nopages := min(iovlen-index, nopages)
	// 		if i, ok := mp[iovno]; ok {
	// 			db.DPrintf(db.LAZYPAGESSRV_FAULT, "extended : %v -> %v+%v", lpc.queue[i].index, index, nopages)
	// 			if lpc.queue[i].index > index {
	// 				lpc.queue[i].addr = uint64(pi * iov.pagesz)
	// 				lpc.queue[i].nopages += lpc.queue[i].index - index
	// 				lpc.queue[i].realaddr = addr
	// 				lpc.queue[i].index = index
	// 			} else if lpc.queue[i].index < index {
	// 				if lpc.queue[i].index+lpc.queue[i].nopages < index+nopages {
	// 					lpc.queue[i].nopages = index + nopages - lpc.queue[i].index
	// 				}

	// 			}
	// 			continue
	// 		}
	// 		mp[iovno] = len(lpc.queue)

	// 		//begin := max(0, index-PREFETCH/4)

	// 		//lpc.queue = append(lpc.queue, Req{iov: iovno, nopages: min(iovlen-begin, PREFETCH/4), index: begin, addr: uint64((pi - (index - begin)) * iov.pagesz), realaddr: addr - uint64(pi*(index-begin))})

	// 		lpc.queue = append(lpc.queue, Req{iov: iovno, nopages: nopages, index: index, addr: uint64(pi * iov.pagesz), realaddr: addr})
	// 	}
	// }
	db.DPrintf(db.LAZYPAGESSRV_FAULT, "QUEUE length %v saved len:%v", len(lpc.queue), len(saved_addresses))
	lpc.mu.Unlock()
	for {
		//	break
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
		//db.DPrintf(db.LAZYPAGESSRV_FAULT, "fetched addr %x hits %v", request.realaddr, lpc.hits)
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
	//oldstartIndex := int(addr0-iov.start) / iov.pagesz
	//startIndex := max(0, oldstartIndex-PREFETCH/2)
	//	addr0 = addr0 - uint64((oldstartIndex-startIndex)*iov.pagesz)
	addr := addr0
	iovlen := int(iov.end-iov.start) / iov.pagesz
	lpc.mu.Lock()
	for ; n < THRESH && addr < iov.end; addr += uint64(iov.pagesz) {

		i := int(addr-iov.start) / iov.pagesz
		if iov.copied[i] {
			break
		}
		// if _, ok := lpc.cache[iovno]; !ok {
		// 	lpc.cache[iovno] = make([]bool, iovlen)
		// }
		exists := lpc.cache[iovno][i]
		// if exists {
		// 	THRESH++
		// }
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
	//db.DPrintf(db.LAZYPAGESSRV_FAULT, "page fault: copy %d: %d(%x) -> %v pi %d(%d,%d,%d) npages(%d)", lpc.nfault, addr, addr, iov, pi, n, nPages(0, uint64(n), lpc.pmi.pagesz), len(buf), lpc.npages)
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
			//iov.copied[startIndex] = true
			off -= int64(iov.pagesz)
		}

		buf = lpc.data[:n*iov.pagesz]
		if newloaded != -1 {
			//		start := time.Now()
			copy(buf[:(newloaded-startIndex)*iov.pagesz], lpc.cachedata[off:off+int64((newloaded-startIndex)*iov.pagesz)])
			//		duration := time.Since(start)
			//		db.DPrintf(db.LAZYPAGESSRV_FAULT, "copy time taken %v pages: %v ", duration.Microseconds(), newloaded-startIndex)
		} else {
			newloaded = startIndex
		}
		//copy(buf[:min(loaded*iov.pagesz, n*iov.pagesz)], lpc.cachedata[off:off+int64(n*iov.pagesz)])
		//start := time.Now()
		if err := lpc.rp(off+int64((newloaded-startIndex)*iov.pagesz), buf[(newloaded-startIndex)*iov.pagesz:n*iov.pagesz]); err != nil {
			db.DFatalf("no page content for %x err: %v", addr, err)
		}
		//duration := time.Since(start)
		//	db.DPrintf(db.LAZYPAGESSRV_FAULT, "rp time taken %v pages: %v", duration.Microseconds(), n-(newloaded-startIndex))
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
				//	iov.copied[startIndex] = true
				off -= int64(iov.pagesz)
			}
			buf = lpc.data[:n*iov.pagesz]
			if newloaded != -1 {
				//	start := time.Now()
				copy(buf[:(newloaded-startIndex)*iov.pagesz], lpc.cachedata[off:off+int64((newloaded-startIndex)*iov.pagesz)])
				//	duration := time.Since(start)
				//	db.DPrintf(db.LAZYPAGESSRV_FAULT, "copy time taken %v pages: %v ", duration.Microseconds(), newloaded-startIndex)
			} else {
				newloaded = startIndex
			}
			//copy(buf[:min(loaded*iov.pagesz, n*iov.pagesz)], lpc.cachedata[off:off+int64(n*iov.pagesz)])
			//	start := time.Now()
			if err := lpc.rp(off+int64((newloaded-startIndex)*iov.pagesz), buf[(newloaded-startIndex)*iov.pagesz:loaded*iov.pagesz]); err != nil {
				db.DFatalf("no page content for %x err: %v", addr, err)
			}
			//	duration := time.Since(start)
			//	db.DPrintf(db.LAZYPAGESSRV_FAULT, "rp time taken %v pages: %v", duration.Microseconds(), loaded-(newloaded-startIndex))
		} else {
			oldStartIndex := startIndex
			for ; startIndex > max(0, oldStartIndex-PREFETCH/2); addr -= uint64(iov.pagesz) {
				//for ; startIndex > 0; addr -= uint64(iov.pagesz) {
				exists := lpc.cache[iovno][startIndex-1]

				if !exists || iov.copied[startIndex-1] {
					break
				}
				startIndex--
				n++
				//	iov.copied[startIndex] = true
				off -= int64(iov.pagesz)
			}
			buf = lpc.data[:n*iov.pagesz]
		}
		//db.DPrintf(db.LAZYPAGESSRV_FAULT, "Preloading! %v-%v: total %v", loaded, n, iovlen-startIndex)
		//	start := time.Now()
		copy(buf[loaded*iov.pagesz:n*iov.pagesz], lpc.cachedata[int64(loaded*iov.pagesz)+off:off+int64(n*iov.pagesz)])
		//	duration := time.Since(start)
		//	db.DPrintf(db.LAZYPAGESSRV_FAULT, "skip copy time taken %v pages: %v ", duration.Microseconds(), n-loaded)
		// for i := loaded; i < n; i++ {

		// 	page, exists := lpc.cache[iovno][i]
		// 	if exists {
		// 		copy(buf[i*iov.pagesz:(i+1)*iov.pagesz], page)
		// 		iov.copied[i] = true
		// 	} else {
		// 		db.DFatalf("unloaded page for %x err: %v", addr, exists)
		// 	}
		// }
	}
	//if ()
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
	if n+startIndex < iovlen {
		lpc.mu.Lock()
		nopages := min(PREFETCH/2, iovlen-n-startIndex)
		// if len(lpc.iovs.iovs) > iovno+1 && iovlen-n-startIndex < PREFETCH/2 {

		// 	iovlen2 := int(lpc.iovs.iovs[iovno+1].end-lpc.iovs.iovs[iovno+1].start) / iov.pagesz
		// 	nopages += min(PREFETCH/2, iovlen2)
		// }
		lpc.queue = append(lpc.queue, Req{iov: iovno, nopages: nopages, index: n + startIndex, addr: uint64(int(off) + n*iov.pagesz)})
		lpc.cond.Signal()
		lpc.mu.Unlock()
	}
	// } else if len(lpc.iovs.iovs) > iovno+1 && n+startIndex == iovlen {
	// 	//db.DPrintf(db.LAZYPAGESSRV_FAULT, "Preload next iov! %v", iovlen-n-startIndex)
	// 	lpc.mu.Lock()
	// 	iovlen2 := int(lpc.iovs.iovs[iovno+1].end-lpc.iovs.iovs[iovno+1].start) / iov.pagesz
	// 	lpc.queue = append(lpc.queue, Req{iov: iovno + 1, nopages: min(PREFETCH/2, iovlen2), index: 0, addr: uint64(int(off) + (iovlen-startIndex)*iov.pagesz)})
	// 	lpc.cond.Signal()
	// 	lpc.mu.Unlock()
	// }
	if startIndex > 0 {
		lpc.mu.Lock()
		nopages := min(PREFETCH/2, startIndex)
		// if iovno > 0 && startIndex < PREFETCH/2 {
		// 	iovlen2 := int(lpc.iovs.iovs[iovno-1].end-lpc.iovs.iovs[iovno-1].start) / iov.pagesz
		// 	nopages = min(PREFETCH/2, iovlen2)
		// 	lpc.queue = append(lpc.queue, Req{iov: iovno - 1, nopages: nopages + startIndex, index: iovlen2 - nopages, addr: uint64(int(off) - (startIndex+nopages)*iov.pagesz)})

		// } else {
		//db.DPrintf(db.LAZYPAGESSRV_FAULT, "Preload prev! %v-%v", startIndex-nopages, startIndex)
		lpc.queue = append(lpc.queue, Req{iov: iovno, nopages: nopages, index: startIndex - nopages, addr: uint64(int(off) - nopages*iov.pagesz)})
		//}
		//db.DPrintf(db.LAZYPAGESSRV_FAULT, "Queued up down %v len: %v\n", iovno, len(lpc.queue))
		lpc.cond.Signal()
		lpc.mu.Unlock()
	}
	//	} else if iovno > 0 {
	// 	lpc.mu.Lock()
	// 	iovlen2 := int(lpc.iovs.iovs[iovno-1].end-lpc.iovs.iovs[iovno-1].start) / iov.pagesz
	// 	nopages := min(PREFETCH/2, iovlen2)
	// 	lpc.queue = append(lpc.queue, Req{iov: iovno - 1, nopages: nopages, index: iovlen2 - nopages, addr: uint64(int(off) - nopages*iov.pagesz)})
	// 	//db.DPrintf(db.LAZYPAGESSRV_FAULT, "Queued up down %v len: %v\n", iovno-1, len(lpc.queue))
	// 	lpc.cond.Signal()
	// 	lpc.mu.Unlock()
	// }
	// if iovno > 0 && iovlen-n-startIndex < PREFETCH/2 {
	// 	lpc.mu.Lock()
	// 	iovlen2 := int(lpc.iovs.iovs[iovno-1].end-lpc.iovs.iovs[iovno-1].start) / iov.pagesz
	// 	nopages := min(PREFETCH/2, iovlen2)
	// 	if _, ok := lpc.cache[iovno-1]; !ok {
	// 		lpc.cache[iovno-1] = make([]bool, iovlen2)
	// 	}
	// 	lpc.queue = append(lpc.queue, Req{iov: iovno - 1, nopages: nopages, index: iovlen2 - nopages, addr: uint64(int(off) - startIndex*iov.pagesz - nopages*iov.pagesz)})

	// 	lpc.mu.Unlock()
	// }
	return n * iov.pagesz, addr
}
func (lpc *lazyPagesConn) pageFault(addr uint64) error {
	//iov := lpc.iovs.find(addr)
	//iov := lpc.iovs.findBinSearch(addr)
	iovno := lpc.iovs.findBinSearch(addr)
	var iov *Iov = nil
	if iovno != -1 {
		iov = lpc.iovs.iovs[iovno]
	}
	// lpc.nfault += 1
	if iov == nil {
		//limitl := addr - uint64(lpc.pmi.pagesz*PREFETCH/2)
		//limitu := addr + uint64(lpc.pmi.pagesz*PREFETCH/2)
		//limitl := addr
		//limitu := addr + uint64(lpc.pmi.pagesz)
		// for _, bound := range lpc.hist {

		// 	if bound.ub <= addr {
		// 		limitl = max(limitl, bound.ub)
		// 		if bound.ub > addr {
		// 			return nil
		// 		}
		// 	} else if bound.lb > addr {
		// 		limitu = min(limitu, bound.lb)
		// 	}
		// }
		lowerbound := addr
		// for ; lowerbound > 0 && lowerbound > limitl; lowerbound -= uint64(lpc.pmi.pagesz) {
		// 	if lpc.iovs.findBinSearch(lowerbound-uint64(lpc.pmi.pagesz)) != -1 {
		// 		break
		// 	}

		// }
		upperbound := addr + uint64(lpc.pmi.pagesz)
		// for ; upperbound < limitu; upperbound += uint64(lpc.pmi.pagesz) {
		// 	if lpc.iovs.findBinSearch(upperbound) != -1 {
		// 		break
		// 	}
		// }

		db.DPrintf(db.LAZYPAGESSRV_FAULT, "page fault: zero %d: no iov for %x, lb: %x ub: %x", lpc.nfault, addr, lowerbound, upperbound)
		//err := zeroLen(lpc.fd, lowerbound, upperbound-lowerbound)
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
			//err = zeroLen(lpc.fd, lowerbound, upperbound-lowerbound)
			cnt += 1
		}
		//	lpc.hist = append(lpc.hist, Range{lb: lowerbound, ub: upperbound})
	} else {
		//addr := iov.start
		//pi := lpc.pmi.find(addr)
		pi := lpc.pmi.findBinSearch(addr)
		addr0 := addr
		if pi == -1 {
			db.DFatalf("no page for %x", addr)
		}
		n, addr := lpc.markAndLoad(iovno, iov, pi, addr)
		//n := iov.markFetchLen(addr)
		if n == 0 {
			db.DPrintf(db.LAZYPAGESSRV_FAULT, "fault page: delivered %d: %d(%x) -> %v pi %d(%d,%d)", lpc.nfault, addr, addr, iovno, pi, n, nPages(0, uint64(n), lpc.pmi.pagesz))
			return nil
		}
		//lpc.sent[iovno] = true
		//lpc.npages += n / 4096
		lpc.npages += 1
		buf := lpc.data[0:n]
		db.DPrintf(db.LAZYPAGESSRV_FAULT, "page fault: copy %d: %x old %d(%x) -> %v pi %d(%d,%d,%d) npages(%d)", lpc.nfault, addr0, addr, addr, iovno, pi, n, nPages(0, uint64(n), lpc.pmi.pagesz), len(buf), lpc.npages)

		// off := int64(pi * lpc.pmi.pagesz)
		// if err := lpc.rp(off, buf); err != nil {
		// 	db.DFatalf("no page content for %x err: %v", addr, err)
		// }

		err := copyPages(lpc.fd, addr, buf)
		cnt := 0
		for err != nil {
			db.DPrintf(db.ERROR, "copyPages err %v try: %v busy:%v ESRCH: %v", err, cnt, err == syscall.EBUSY, err == syscall.ESRCH)
			if err == syscall.ESRCH {
				return nil
			}
			// if err == syscall.ENOENT {

			// 	if n <= lpc.pmi.pagesz {
			// 		return err
			// 	}
			// 	n -= lpc.pmi.pagesz
			// 	iov.unmarkFetchLen(addr+uint64(n), 1)
			// 	buf = lpc.data[0:n]
			// 	off := int64(pi * lpc.pmi.pagesz)
			// 	if err := lpc.rp(off, buf); err != nil {
			// 		db.DFatalf("no page content for %x", addr)
			// 	}
			// 	db.DPrintf(db.LAZYPAGESSRV_FAULT, "page fault: copy %x: %d(%x) -> %v pi %d(%d,%d,%d)", lpc.nfault, addr, addr, iov, pi, n, nPages(0, uint64(n), lpc.pmi.pagesz), len(buf))
			// 	err = copyPages(lpc.fd, addr, buf)
			// 	//		iov.unmarkFetchLen(addr, n)

			// 	continue
			// }
			if cnt > 10 || (err != syscall.EBUSY && err != syscall.ENOENT && err != syscall.EAGAIN) {

				return err
			}
			if err == syscall.EAGAIN {
				//	iov.unmarkFetchLen(addr, n)
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
		// if err != nil {
		// 	db.DPrintf(db.ERROR, "SYS_IOCTL1 err %v", err)
		// 	// 	time.Sleep(1000 * time.Millisecond)
		// 	// 	copyPages(lpc.fd, addr, buf)

		//}
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
