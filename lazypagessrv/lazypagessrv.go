// The lazypagesrv package implements a go version of criu's
// lazy-pages daemon, but specialized to sigmaos's needs.
package lazypagessrv

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/userfaultfd"
)

const SIGMAOS = true

const SOCKNAME = "lazy-pages.socket"

// XXX imgdir and pages should per LazyPagesConn
type LazyPagesSrv struct {
	*sigmaclnt.SigmaClnt
	imgdir string
	pages  string
	pagesz int
}

func NewLazyPagesSrv(imgdir, pages string) (*LazyPagesSrv, error) {
	lps := &LazyPagesSrv{
		imgdir: imgdir,
		pages:  pages,
		pagesz: os.Getpagesize()}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	lps.SigmaClnt = sc
	return lps, nil
}

func (lps *LazyPagesSrv) Run() error {
	sn := filepath.Join(lps.imgdir, SOCKNAME)
	socket, err := net.Listen("unix", sn)
	if err != nil {
		db.DFatalf("Listen: err %v", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Remove(sn)
		os.Exit(1)
	}()

	for {
		conn, err := socket.Accept()
		if err != nil {
			db.DFatalf("Accept: err %v", err)
		}
		go lps.handleConn(conn)
	}
}

func (lps *LazyPagesSrv) handleConn(conn net.Conn) {
	defer conn.Close()

	connFd, err := getConnFd(conn.(*net.UnixConn))
	if err != nil {
		db.DFatalf("getConnFd err %v", err)
	}

	var pid int32
	if err := binary.Read(conn, binary.NativeEndian, &pid); err != nil {
		db.DFatalf("Read pid err %v", err)
	}

	db.DPrintf(db.LAZYPAGESSRV, "pid %d\n", pid)

	b := make([]byte, unix.CmsgSpace(4))
	_, _, _, _, err = unix.Recvmsg(connFd, nil, b, 0)
	if err != nil {
		db.DFatalf("Recvmsg err %v", err)
	}
	// parse socket control message
	cmsgs, err := unix.ParseSocketControlMessage(b)
	if err != nil {
		db.DFatalf("ParseSocketControlMessage err %v", err)
	}
	fds, err := unix.ParseUnixRights(&cmsgs[0])
	if err != nil {
		db.DFatalf("ParseUnixRights err %v", err)
	}
	fd := fds[0]
	db.DPrintf(db.LAZYPAGESSRV, "Received fd %d\n", fd)

	var rp func(int64, []byte) error
	if SIGMAOS {
		fdpages, err := lps.Open(lps.pages, sp.OREAD)
		if err != nil {
			db.DPrintf(db.LAZYPAGESSRV, "Open %v err %v\n", lps.pages, err)
			return
		}
		defer lps.CloseFd(fd)
		rp = func(off int64, pages []byte) error {
			return readBytesSigma(lps.SigmaClnt, fdpages, off, pages)
		}
	} else {
		f, err := os.Open(lps.pages)
		if err != nil {
			db.DPrintf(db.LAZYPAGESSRV, "Open %v err %v\n", lps.pages, err)
			return
		}
		defer f.Close()
		rp = func(off int64, pages []byte) error {
			return readBytes(f, off, pages)
		}
	}

	lpc, err := lps.newLazyPagesConn(fd, rp, int(pid))
	if err != nil {
		db.DFatalf("newLazyPagesConn fd %v err %v", fd, err)
	}
	if err := lpc.handleReqs(); err != nil {
		db.DFatalf("handle fd %v err %v", fd, err)
	}
}

type LazyPagesConn struct {
	lps       *LazyPagesSrv
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

func (lps *LazyPagesSrv) newLazyPagesConn(fd int, rp func(int64, []byte) error, pid int) (*LazyPagesConn, error) {
	lpc := &LazyPagesConn{lps: lps, fd: fd, rp: rp, pid: int(pid)}
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

func (lpc *LazyPagesConn) handleReqs() error {
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
			return fmt.Errorf("Unknown event %v", userfaultfd.GetMsgEvent(&msg))
		}
	}
}

func (lpc *LazyPagesConn) pageFault(addr uint64) error {
	iov := lpc.iovs.find(addr)
	lpc.nfault += 1
	if iov == nil {
		db.DPrintf(db.LAZYPAGESSRV, "page fault %d: no iov for %x", lpc.nfault, addr)
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

func readBytes(f *os.File, off int64, page []byte) error {
	if _, err := f.Seek(off, 0); err != nil {
		return err
	}
	if _, err := f.Read(page); err != nil {
		return err
	}
	return nil
}

func getConnFd(conn syscall.Conn) (connFd int, err error) {
	var rawConn syscall.RawConn
	rawConn, err = conn.SyscallConn()
	if err != nil {
		return
	}
	err = rawConn.Control(func(fd uintptr) {
		connFd = int(fd)
	})
	return
}
