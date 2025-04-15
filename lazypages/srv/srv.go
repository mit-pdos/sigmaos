// The lazypagesrv package implements a go version of criu's
// lazy-pages daemon, but specialized to sigmaos's needs.
package srv

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/lazypages"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/syncmap"
)

type lazyPagesSrv struct {
	*sigmaclnt.SigmaClnt
	pids   *syncmap.SyncMap[int, *lazyPagesConn]
	pagesz int
}

func newLazyPagesSrv(sc *sigmaclnt.SigmaClnt) (*lazyPagesSrv, error) {
	lps := &lazyPagesSrv{
		SigmaClnt: sc,
		pids:      syncmap.NewSyncMap[int, *lazyPagesConn](),
		pagesz:    os.Getpagesize(),
	}
	return lps, nil
}

// Called indirectly from ExecLazyPagesSrv
func Run() error {
	pe := proc.GetProcEnv()
	pid := pe.GetPID()

	db.DPrintf(db.ALWAYS, "Run: lazypagessrv %v", lazypages.SrvPath(pid))

	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}

	lps, err := newLazyPagesSrv(sc)
	if err != nil {
		db.DFatalf("Error newlazyPagesSrv: %v", err)
	}

	svc := &LazyPagesSvc{lps}
	_, err = sigmasrv.NewSigmaSrvClnt(lazypages.SrvPath(pid), sc, svc)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}

	go func() {
		if err := lps.servePages(pid); err != nil {
			db.DPrintf(db.ERROR, "servePages err %v", err)
		}
	}()

	// Tell ExecLazyPagesSrv we are running
	if _, err := io.WriteString(os.Stdout, "r"); err != nil {
		db.DPrintf(db.ALWAYS, "WriteString: err %w", err)
		os.Exit(1)
	}

	lps.waitExit()
	return nil
}

func (lps *lazyPagesSrv) waitExit() {
	buf := make([]byte, 1)
	if _, err := io.ReadFull(os.Stdin, buf); err != nil {
		db.DFatalf("read pipe err %v\n", err)
	}
	db.DPrintf(db.LAZYPAGESSRV, "exiting\n")
	os.Exit(0)
}

func (lps *lazyPagesSrv) servePages(pid sp.Tpid) error {
	sn := filepath.Join(lazypages.WorkDir(pid), lazypages.SOCKNAME)
	os.Mkdir(lazypages.DIR, 0755)
	if err := os.Mkdir(lazypages.WorkDir(pid), 0755); err != nil {
		return err
	}
	socket, err := net.Listen("unix", sn)
	if err != nil {
		db.DFatalf("Listen: err %v", err)
	}

	for {
		conn, err := socket.Accept()
		if err != nil {
			db.DFatalf("Accept: err %v", err)
		}
		go lps.handleConn(conn)
	}
}

func (lps *lazyPagesSrv) handleConn(conn net.Conn) {
	defer conn.Close()

	connFd, err := getConnFd(conn.(*net.UnixConn))
	if err != nil {
		db.DFatalf("getConnFd err %v", err)
	}

	var pid int32
	if err := binary.Read(conn, binary.NativeEndian, &pid); err != nil {
		db.DFatalf("Read pid err %v", err)
	}

	// db.DPrintf(db.LAZYPAGESSRV, "pid %d\n", pid)
	flags, err := unix.FcntlInt(uintptr(connFd), unix.F_GETFL, 0)
	newFlags := flags &^ unix.O_NONBLOCK
	_, err = unix.FcntlInt(uintptr(connFd), unix.F_SETFL, newFlags)
	b := make([]byte, unix.CmsgSpace(4))
	_, _, _, _, err = unix.Recvmsg(connFd, nil, b, 0)
	cnt := 0

	for err != nil {
		db.DPrintf(db.ERROR, "Recvmsg err %v try: %v", err, cnt)
		if cnt > 10 || err != syscall.EAGAIN {
			db.DFatalf("Recvmsg err %v", err)
		}
		time.Sleep(100 * time.Millisecond)
		_, _, _, _, err = unix.Recvmsg(connFd, nil, b, 0)
		cnt += 1
	}
	// if err != nil {

	// 	db.DFatalf("Recvmsg err %v", err)
	// }
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
	lpc, ok := lps.pids.Lookup(int(pid))
	if !ok {
		db.DFatalf("newLazyPagesConn pid %v no registration", pid, err)
	}

	fdpages, err := lps.Open(lpc.pages, sp.OREAD)
	if err != nil {
		db.DPrintf(db.LAZYPAGESSRV, "Open %v err %v\n", lpc.pages, err)
		return
	}
	defer lps.CloseFd(fd)
	rp := func(off int64, pages []byte) error {
		return readBytesSigma(lps.SigmaClnt, fdpages, off, pages)
	}
	lpc.fd = fd
	lpc.rp = rp
	if err := lpc.handleReqs(); err != nil {
		db.DFatalf("handle pid %v err %v", pid, err)
	}
	lps.pids.Delete(int(pid))
}

func (lps *lazyPagesSrv) register(pid int, imgdir, pages string) error {
	lpc, err := lps.newLazyPagesConn(pid, imgdir, pages)
	if err != nil {
		return err
	}
	ok := lps.pids.Insert(pid, lpc)
	db.DPrintf(db.LAZYPAGESSRV, "registering %v", pid)
	if !ok {
		db.DFatalf("Insert: exists %d", pid)
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
