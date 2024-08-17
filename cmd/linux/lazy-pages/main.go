package main

import (
	"encoding/binary"
	// "encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/checkpoint-restore/go-criu/v7/crit"
	"github.com/checkpoint-restore/go-criu/v7/crit/cli"
	"github.com/checkpoint-restore/go-criu/v7/crit/images/mm"
	"github.com/checkpoint-restore/go-criu/v7/crit/images/pagemap"
	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/userfaultfd"
)

const SOCKNAME = "lazy-pages.socket"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <image-dir>\n", os.Args[0])
		os.Exit(1)
	}
	db.DPrintf(db.ALWAYS, "lazy-pages: %v", os.Args[1])

	sn := filepath.Join(os.Args[1], SOCKNAME)
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

			db.DPrintf(db.ALWAYS, "pid %d\n", pid)

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
			db.DPrintf(db.ALWAYS, "Received fd %d\n", fd)
			if err := handleReqs(int(pid), fd); err != nil {
				db.DPrintf(db.ALWAYS, "handle fd %v err\n", fd, err)
			}
		}(conn)
	}
}

func readImg(pid int, magic string) (*crit.CriuImage, error) {
	// f, err := os.Open(filepath.Join(imgDir, "inventory.img"))
	pn := filepath.Join(os.Args[1], magic+"-"+strconv.Itoa(int(pid))+".img")
	f, err := os.Open(pn)
	if err != nil {
		return nil, err
	}
	c := crit.New(f, nil, "", false, false)
	entryType, err := cli.GetEntryTypeFromImg(f)
	if err != nil {
		return nil, fmt.Errorf("Unknown Entry type %q: %w", pn, err)
	}
	img, err := c.Decode(entryType)
	if err != nil {
		return nil, err
	}
	return img, nil
}

type TpagemapImg struct {
	PageMapHead    *crit.CriuEntry
	PagemapEntries []*crit.CriuEntry
}

func newTpagemapImg(pid int) (*TpagemapImg, error) {
	img, err := readImg(pid, "pagemap")
	if err != nil {
		return nil, err
	}
	return &TpagemapImg{PageMapHead: img.Entries[0], PagemapEntries: img.Entries[1:]}, nil
}

func (pmi *TpagemapImg) find(addr uint64) int {
	pi := 0
	for _, pme := range pmi.PagemapEntries {
		pm := pme.Message.(*pagemap.PagemapEntry)
		start := pm.GetVaddr()
		n := pm.GetNrPages()
		end := start + uint64(n*uint32(os.Getpagesize()))
		if addr >= start && addr < end {
			m := (addr - start) / uint64(os.Getpagesize())
			pi = pi + int(m)
			db.DPrintf(db.ALWAYS, "m %d pi %d\n", m, pi)
			return pi
		}
		pi += int(n)
	}
	return -1
}

func (pmi *TpagemapImg) read(pid, pi int) ([]byte, error) {
	page := make([]byte, os.Getpagesize())
	ph := pmi.PageMapHead.Message.(*pagemap.PagemapHead)
	if err := readPage(pid, int(ph.GetPagesId()), pi, page); err != nil {
		return nil, err
	}
	return page, nil
}

type Tmm struct {
	*mm.MmEntry
}

func newTmm(pid int) (*Tmm, error) {
	img, err := readImg(pid, "mm")
	if err != nil {
		return nil, err
	}
	return &Tmm{MmEntry: img.Entries[0].Message.(*mm.MmEntry)}, nil
}

type Iov struct {
	start     uint64
	end       uint64
	img_start uint64
}

func (iov *Iov) String() string {
	return fmt.Sprintf("{%x %x %x}", iov.start, iov.end, iov.img_start)
}

type Iovs struct {
	iovs []*Iov
}

func newIovs() *Iovs {
	return &Iovs{iovs: make([]*Iov, 0)}
}

func (iovs *Iovs) append(iov *Iov) {
	iovs.iovs = append(iovs.iovs, iov)
}

func (iovs *Iovs) find(addr uint64) *Iov {
	for _, iov := range iovs.iovs {
		if iov.start <= addr && addr < iov.end {
			return iov
		}
	}
	return nil
}

func (mm *Tmm) collectIovs(pmi *TpagemapImg) *Iovs {
	db.DPrintf(db.TEST, "mmInfo %d\n", len(mm.Vmas))

	iovs := newIovs()
	end := uint64(os.Getpagesize())
	start := uint64(0)
	nPages := uint32(0)
	max_iov_len := start

	ph := pmi.PageMapHead.Message.(*pagemap.PagemapHead)
	db.DPrintf(db.TEST, "ph %v", ph)

	for _, pme := range pmi.PagemapEntries[1:] {
		pm := pme.Message.(*pagemap.PagemapEntry)

		db.DPrintf(db.TEST, "pm %v", pm)

		start = pm.GetVaddr()
		end = start + uint64(pm.GetNrPages()*uint32(os.Getpagesize()))
		nPages += pm.GetNrPages()

		for _, vma := range mm.Vmas {
			if start >= vma.GetStart() {
				continue
			}
			iov := &Iov{}
			vend := vma.GetEnd()
			len := end
			if vend < end {
				end = vend
			}
			len = len - start
			iov.start = start
			iov.img_start = start
			iov.end = iov.start + len
			iovs.append(iov)

			if len > max_iov_len {
				max_iov_len = len
			}

			if end < vend {
				db.DPrintf(db.TEST, "%d vma %v\n", end, vma)
				break
			}
			start = vend
		}
	}
	// XXX do something with max_iov_len
	db.DPrintf(db.TEST, "max_iov_len %d\n", max_iov_len)
	return iovs
}

func readPage(pid, pageId, pi int, page []byte) error {
	pn := filepath.Join(os.Args[1], "pages-"+strconv.Itoa(pageId)+".img")
	f, err := os.Open(pn)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(int64(pi*os.Getpagesize()), 0); err != nil {
		return err
	}
	if _, err := f.Read(page); err != nil {
		return err
	}
	return nil
}

func handleReqs(pid, fd int) error {
	pmi, err := newTpagemapImg(pid)
	if err != nil {
		return err
	}
	mm, err := newTmm(pid)
	if err != nil {
		return err
	}
	iovs := mm.collectIovs(pmi)
	db.DPrintf(db.TEST, "iovs: %v", iovs)
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
			return fmt.Errorf("Unknown event %v", userfaultfd.GetMsgEvent(&msg))
		}

		arg := userfaultfd.GetMsgArg(&msg)
		pagefault := (*(*userfaultfd.UffdPagefault)(unsafe.Pointer(&arg[0])))

		addr := uint64(userfaultfd.GetPagefaultAddress(&pagefault))
		mask := uint64(^(os.Getpagesize() - 1))
		db.DPrintf(db.ALWAYS, "page fault %d %x %x", addr, addr, addr&mask)
		addr = addr & mask
		iov := iovs.find(addr)
		if iov == nil {
			db.DPrintf(db.ALWAYS, "no iov for %x", addr)
			zeroPage(fd, addr)
		} else {
			pi := pmi.find(addr)
			if pi == -1 {
				db.DFatalf("no page for %x", addr)
			}
			db.DPrintf(db.ALWAYS, "page fault %d -> %d", addr, pi)
			page, err := pmi.read(pid, pi)
			if err != nil {
				db.DFatalf("no page content for %x", addr)
			}
			copyPage(fd, addr, page)
		}
	}
}

func zeroPage(fd int, addr uint64) {
	len := uint64(os.Getpagesize())
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

func copyPage(fd int, addr uint64, page []byte) {
	pagesize := os.Getpagesize()
	cpy := userfaultfd.NewUffdioCopy(
		page,
		userfaultfd.CULong(addr),
		userfaultfd.CULong(pagesize),
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
