package container

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"google.golang.org/protobuf/proto"

	criu "github.com/checkpoint-restore/go-criu/v7"
	"github.com/checkpoint-restore/go-criu/v7/crit/images/fdinfo"
	sk_unix "github.com/checkpoint-restore/go-criu/v7/crit/images/sk-unix"
	"github.com/checkpoint-restore/go-criu/v7/rpc"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/lazypagessrv"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type NoNotify struct {
	criu.NoNotify
}

func CheckpointProc(c *criu.Criu, pid int, imgDir string, spid sp.Tpid, ino uint64) error {
	db.DPrintf(db.CKPT, "CheckpointProc %q %v ino %d", imgDir, pid, ino)
	img, err := os.Open(imgDir)
	if err != nil {
		db.DPrintf(db.CKPT, "CheckpointProc: error opening img dir %v", err)
		return err
	}
	defer img.Close()

	verbose := db.IsLabelSet(db.CRIU)
	root := "/home/sigmaos/jail/" + spid.String() + "/"
	extino := "unix[" + strconv.FormatInt(int64(ino), 10) + "]"
	opts := &rpc.CriuOpts{
		Pid:          proto.Int32(int32(pid)),
		ImagesDirFd:  proto.Int32(int32(img.Fd())),
		Root:         proto.String(root),
		TcpClose:     proto.Bool(true), // XXX does it matter on dump?
		External:     []string{extino, "mnt[/lib]:libMount", "mnt[/lib64]:lib64Mount", "mnt[/usr]:usrMount", "mnt[/etc]:etcMount", "mnt[/bin]:binMount", "mnt[/dev]:devMount", "mnt[/tmp]:tmpMount", "mnt[/tmp/sigmaos-perf]:perfMount", "mnt[/mnt]:mntMount", "mnt[/mnt/binfs]:binfsMount"},
		Unprivileged: proto.Bool(true),
		ExtUnixSk:    proto.Bool(true),
	}
	if verbose {
		opts.LogLevel = proto.Int32(4)
		opts.LogFile = proto.String("dump.log")
	}
	err = c.Dump(opts, NoNotify{})
	db.DPrintf(db.CKPT, "CheckpointProc: dump err %v", err)
	if verbose {
		dumpLog(imgDir + "/dump.log")
	}
	if err != nil {
		return err
	}
	if LAZY {
		if err := lazypagessrv.FilterLazyPages(imgDir); err != nil {
			db.DPrintf(db.CKPT, "CheckpointProc: DumpNonLazyPages err %v", err)
			return err
		}
	}
	return nil
}

func mkMount(mnt, dst, t string, flags uintptr) error {
	os.Mkdir(dst, 0755)
	if err := syscall.Mount(mnt, dst, t, flags, ""); err != nil {
		db.DPrintf(db.CKPT, "Mount mnt %s dst %s t %s err %v", mnt, dst, t, err)
		return err
	}
	return nil
}

func restoreMounts(sigmaPid sp.Tpid) error {
	// create dir for proc to be put in
	jailPath := "/home/sigmaos/jail/" + sigmaPid.String() + "/"
	os.Mkdir(jailPath, 0777)

	// Mount /lib
	if err := mkMount("/lib", jailPath+"/lib", "none", syscall.MS_BIND|syscall.MS_RDONLY); err != nil {
		return err
	}
	if err := mkMount("/lib64", jailPath+"/lib64", "none", syscall.MS_BIND|syscall.MS_RDONLY); err != nil {
		return err
	}
	if err := mkMount(jailPath+"/proc", jailPath+"/proc", "proc", syscall.MS_BIND|syscall.MS_RDONLY); err != nil {
		return err
	}
	// Mount realm's user bin directory as /bin
	if err := mkMount(filepath.Join(sp.SIGMAHOME, "bin/user"), jailPath+"/bin", "none", syscall.MS_BIND|syscall.MS_RDONLY); err != nil {
		return err
	}
	if err := mkMount("/usr", jailPath+"/usr", "none", syscall.MS_BIND|syscall.MS_RDONLY); err != nil {
		return err
	}
	// Mount /dev/urandom
	if err := mkMount("/dev", jailPath+"dev", "none", syscall.MS_BIND|syscall.MS_RDONLY); err != nil {
		return err
	}
	if err := mkMount("/etc", jailPath+"etc", "none", syscall.MS_BIND|syscall.MS_RDONLY); err != nil {
		return err
	}
	if err := mkMount("/tmp", jailPath+"tmp", "none", syscall.MS_BIND|syscall.MS_RDONLY); err != nil {
		return err
	}
	// Mount perf dir
	os.Mkdir(jailPath+"tmp", 0755)
	if err := mkMount("/tmp/sigmaos-perf", jailPath+"tmp/sigmaos-perf", "none", syscall.MS_BIND); err != nil {
		return err
	}
	if err := mkMount("/mnt", jailPath+"mnt", "none", syscall.MS_BIND|syscall.MS_RDONLY); err != nil {
		return err
	}
	// Mount /mnt/binfs
	if err := syscall.Mount("/mnt/binfs", jailPath+"mnt/binfs", "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		return err
	}

	return nil
}

func RestoreProc(criuclnt *criu.Criu, p *proc.Proc, imgDir, workDir string) error {
	db.DPrintf(db.CKPT, "RestoreProc %v %v %v", imgDir, workDir, p)
	if err := restoreMounts(p.GetPid()); err != nil {
		return err
	}
	jailPath := "/home/sigmaos/jail/" + p.GetPid().String() + "/"
	return restoreProc(criuclnt, p, imgDir, workDir, jailPath)
}

func restoreProc(criuclnt *criu.Criu, proc *proc.Proc, imgDir, workDir, jailPath string) error {
	img, err := os.Open(imgDir)
	if err != nil {
		db.DPrintf(db.CKPT, "restoreProc: Open %v err", imgDir, err)
		return err
	}
	defer img.Close()

	wd, err := os.Open(workDir)
	if err != nil {
		db.DPrintf(db.CKPT, "restoreProc: Open %v err", workDir, err)
		return err
	}
	defer wd.Close()

	verbose := db.IsLabelSet(db.CRIU)

	// XXX deduplicate with GetNetproxydConn
	conn, err := net.Dial("unix", sp.SIGMA_NETPROXY_SOCKET)
	if err != nil {
		db.DFatalf("Error connect netproxy srv %v err %v", sp.SIGMA_NETPROXY_SOCKET, err)
	}
	uconn := conn.(*net.UnixConn)
	b, err := json.Marshal(proc.GetPrincipal())
	if err != nil {
		db.DFatalf("Error marshal principal: %v", err)
		return err
	}
	// Write the principal ID to the server, so that the server
	// knows the principal associated with this connection. For non-test
	// programs, this will be done by the trampoline.
	if err := frame.WriteFrame(uconn, b); err != nil {
		db.DPrintf(db.ERROR, "Error WriteFrame principal: %v", err)
		return err
	}

	rdr, wrt, err := newSocketPair()
	if err != nil {
		db.DFatalf("unixPair err %v\n", err)
	}

	// XXX where does 2 come from?
	criuDump, err := lazypagessrv.ReadImg(imgDir, "2", "fdinfo")
	if err != nil {
		db.DFatalf("ReadImg fdinfo err %v\n", err)
	}
	// XXX 3 is SIGMA_NETPROXY_FD
	dstfd := criuDump.Entries[3].Message.(*fdinfo.FdinfoEntry)
	criuDump, err = lazypagessrv.ReadImg(imgDir, "", "files")
	if err != nil {
		db.DFatalf("ReadImg files err %v\n", err)
	}
	var usk *sk_unix.UnixSkEntry
	for _, f := range criuDump.Entries {
		e := f.Message.(*fdinfo.FileEntry)
		if e.GetId() == dstfd.GetId() {
			usk = e.GetUsk()
		}
	}
	if usk == nil {
		db.DFatalf("ReadImg usk err %v\n", err)
	}
	inostr := "socket:[" + strconv.Itoa(int(usk.GetIno())) + "]"
	fd := int32(rdr.Fd())
	ifd := &rpc.InheritFd{Fd: &fd, Key: &inostr}
	ifds := []*rpc.InheritFd{ifd}

	db.DPrintf(db.CKPT, "Invoke restore with fd %d dstfd %v key %v\n", fd, dstfd, inostr)

	opts := &rpc.CriuOpts{
		ImagesDirFd: proto.Int32(int32(img.Fd())),
		WorkDirFd:   proto.Int32(int32(wd.Fd())),
		Root:        proto.String(jailPath),
		TcpClose:    proto.Bool(true),
		External:    []string{"mnt[libMount]:/lib", "mnt[lib64Mount]:/lib64", "mnt[usrMount]:/usr", "mnt[etcMount]:/etc", "mnt[binMount]:/home/sigmaos/bin/user", "mnt[devMount]:/dev", "mnt[tmpMount]:/tmp", "mnt[perfMount]:/tmp/sigmaos-perf", "mnt[mntMount]:/mnt", "mnt[binfsMount]:/mnt/binfs"},
		LazyPages:   proto.Bool(LAZY),
		InheritFd:   ifds,
	}
	if verbose {
		opts.LogLevel = proto.Int32(4)
		opts.LogFile = proto.String("restore.log")
	}

	go func() {
		err = criuclnt.Restore(opts, nil)
		db.DPrintf(db.CKPT, "restoreProc: Restore err %v", err)
		if verbose {
			dumpLog(workDir + "/restore.log")
		}
	}()

	// wait for restored proc to check in
	b = make([]byte, 1)
	if _, err := wrt.Read(b); err != nil {
		db.DFatalf("sendConn err %v\n", err)
	}

	db.DPrintf(db.CKPT, "restored proc is running")

	if err := sendConn(wrt, uconn); err != nil {
		db.DFatalf("sendConn err %v\n", err)
	}

	if err := sendProcEnv(wrt, proc); err != nil {
		db.DFatalf("sendConn err %v\n", err)
	}

	db.DPrintf(db.CKPT, "sendConn: sent")

	return nil
}

func dumpLog(pn string) error {
	b, err := os.ReadFile(pn)
	if err != nil {
		db.DPrintf(db.CKPT, "ReadFile %q err %v", pn, err)
		return err
	}
	db.DPrintf(db.CKPT, "dumpLog %q: %s", pn, string(b))
	return nil
}

func ino(f *os.File) uint64 {
	st := syscall.Stat_t{}
	fd := int(f.Fd())
	if err := syscall.Fstat(fd, &st); err != nil {
		db.DFatalf("fstat %v\n", err)
	}
	return st.Ino
}

func newSocketPair() (*os.File, *os.File, error) {
	fd, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, err
	}
	src := os.NewFile(uintptr(fd[0]), "src")
	dst := os.NewFile(uintptr(fd[1]), "dst")
	return src, dst, nil
}

func sendConn(wrt *os.File, uconn *net.UnixConn) error {
	conn, err := net.FileConn(wrt)
	if err != nil {
		db.DFatalf("sndConn: FileConn err %v", err)
	}
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		db.DFatalf("sndConn: unixConn err %v", err)
	}
	return sndConn(unixConn, uconn)
}

func sndConn(wrt *net.UnixConn, uconn *net.UnixConn) error {
	file, err := uconn.File()
	if err != nil {
		return err
	}
	oob := syscall.UnixRights(int(file.Fd()))
	_, _, err = wrt.WriteMsgUnix(nil, oob, nil)
	return err
}

func sendProcEnv(wrt *os.File, p *proc.Proc) error {
	conn, err := net.FileConn(wrt)
	if err != nil {
		db.DFatalf("sndConn: FileConn err %v", err)
	}
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		db.DFatalf("sndConn: unixConn err %v", err)
	}
	b, err := json.Marshal(proc.NewProcEnvFromProto(p.ProcEnvProto))
	if err != nil {
		return err
	}
	if err := frame.WriteFrame(unixConn, b); err != nil {
		db.DPrintf(db.ERROR, "sendProc: WriteFrame err%v", err)
		return err
	}
	return err
}
