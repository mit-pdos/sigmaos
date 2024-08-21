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

type LazyPagesSrv struct {
	*sigmaclnt.SigmaClnt
	imgdir   string
	pagesdir string
	pages    string
	pagesz   int
}

func NewLazyPagesSrv(imgdir, pagesdir, pages string) (*LazyPagesSrv, error) {
	lps := &LazyPagesSrv{
		imgdir:   imgdir,
		pagesdir: pagesdir,
		pages:    pages,
		pagesz:   os.Getpagesize()}
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
		go func(conn net.Conn) {
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
			if err := lps.handleReqs(int(pid), fd); err != nil {
				db.DFatalf("handle fd %v err %v", fd, err)
			}
		}(conn)
	}
}

func (lps *LazyPagesSrv) handleReqs(pid, fd int) error {
	pmi, err := newTpagemapImg(lps.imgdir, pid)
	if err != nil {
		return err
	}
	mm, err := newTmm(lps.imgdir, pid)
	if err != nil {
		return err
	}
	iovs, npages, maxIovLen := mm.collectIovs(pmi)
	nfault := 0
	page := make([]byte, pmi.pagesz) // XXX maxIovLen
	db.DPrintf(db.ALWAYS, "lazypages: img %v pagesdir %v pages %v pid %d fd %d iovs %d npages %d maxIovLen %d", lps.imgdir, lps.pagesdir, lps.pages, pid, fd, iovs.len(), npages, maxIovLen)
	for {
		if _, err := unix.Poll(
			[]unix.PollFd{{
				Fd:     int32(fd),
				Events: unix.POLLIN,
			}},
			-1,
		); err != nil {
			return err
		}
		buf := make([]byte, unsafe.Sizeof(userfaultfd.UffdMsg{}))
		if _, err := syscall.Read(fd, buf); err != nil {
			return err
		}
		msg := (*(*userfaultfd.UffdMsg)(unsafe.Pointer(&buf[0])))
		if userfaultfd.GetMsgEvent(&msg) != userfaultfd.UFFD_EVENT_PAGEFAULT {
			db.DPrintf(db.ALWAYS, "Unknown even %v", userfaultfd.GetMsgEvent(&msg))
			return fmt.Errorf("Unknown event %v", userfaultfd.GetMsgEvent(&msg))
		}

		arg := userfaultfd.GetMsgArg(&msg)
		pagefault := (*(*userfaultfd.UffdPagefault)(unsafe.Pointer(&arg[0])))
		addr := uint64(userfaultfd.GetPagefaultAddress(&pagefault))
		mask := uint64(^(lps.pagesz - 1))
		addr = addr & mask

		iov := iovs.find(addr)
		nfault += 1
		if iov == nil {
			db.DPrintf(db.LAZYPAGESSRV, "page fault %d: no iov for %x", nfault, addr)
			lps.zeroPage(fd, addr)
		} else {
			// XXX read and copy the whole iov instead of one?
			pi := pmi.find(addr)
			if pi == -1 {
				db.DFatalf("no page for %x", addr)
			}
			db.DPrintf(db.LAZYPAGESSRV, "page fault %d: %d(%x) -> %v %d", nfault, addr, addr, iov, pi)
			off := int64(pi * pmi.pagesz)
			if err := lps.readPage(lps.pages, off, page); err != nil {
				db.DFatalf("no page content for %x", addr)
			}
			lps.copyPage(fd, addr, page)
		}
	}
}

func (lps *LazyPagesSrv) zeroPage(fd int, addr uint64) {
	len := uint64(lps.pagesz)
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
		db.DFatalf("SYS_IOCTL err %v", errno)
	}
}

func (lps *LazyPagesSrv) copyPage(fd int, addr uint64, page []byte) {
	cpy := userfaultfd.NewUffdioCopy(
		page,
		userfaultfd.CULong(addr),
		userfaultfd.CULong(lps.pagesz),
		0,
		0,
	)
	if _, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		userfaultfd.UFFDIO_COPY,
		uintptr(unsafe.Pointer(&cpy)),
	); errno != 0 {
		db.DFatalf("SYS_IOCTL err %v", errno)
	}
}

func (lps *LazyPagesSrv) readPage(pn string, off int64, page []byte) error {
	if SIGMAOS {
		fd, err := lps.Open(pn, sp.OREAD)
		if err != nil {
			return err
		}
		defer lps.CloseFd(fd)
		if _, err := lps.Pread(fd, page, sp.Toffset(off)); err != nil {
			return err
		}
	} else {
		f, err := os.Open(pn)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.Seek(off, 0); err != nil {
			return err
		}
		if _, err := f.Read(page); err != nil {
			return err
		}
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
