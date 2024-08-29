package container

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

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
	"sigmaos/uprocsrv/binsrv"
)

const (
	LAZY = true
)

type uprocCmd struct {
	cmd *exec.Cmd
	ino uint64
}

func (upc *uprocCmd) Wait() error {
	return upc.cmd.Wait()
}

func (upc *uprocCmd) Pid() int {
	return upc.cmd.Process.Pid
}

func (upc *uprocCmd) Ino() uint64 {
	return upc.ino
}

// Contain user procs using exec-uproc-rs trampoline
func StartUProc(uproc *proc.Proc, netproxy bool) (*uprocCmd, error) {
	var cmd *exec.Cmd
	straceProcs := proc.GetLabels(uproc.GetProcEnv().GetStrace())

	pn := binsrv.BinPath(uproc.GetVersionedProgram())
	db.DPrintf(db.CONTAINER, "StartUProc %q netproxy %v %v env %v\n", pn, netproxy, uproc, os.Environ())

	// Optionally strace the proc
	if straceProcs[uproc.GetProgram()] {
		cmd = exec.Command("strace", append([]string{"-D", "-f", "exec-uproc-rs", uproc.GetPid().String(), pn, strconv.FormatBool(netproxy)}, uproc.Args...)...)
	} else {
		cmd = exec.Command("exec-uproc-rs", append([]string{uproc.GetPid().String(), pn, strconv.FormatBool(netproxy)}, uproc.Args...)...)
	}
	uproc.AppendEnv("PATH", "/bin:/bin2:/usr/bin:/home/sigmaos/bin/kernel")
	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))
	uproc.AppendEnv(proc.SIGMAPERF, uproc.GetProcEnv().GetPerf())
	// uproc.AppendEnv("RUST_BACKTRACE", "1")
	cmd.Env = uproc.GetEnv()

	f, err := os.Create("/tmp/sigmaos-perf/log-proc.txt")
	if err != nil {
		db.DFatalf("Error creating %v\n", err)
	}

	rdr, _, err := newSocketPair()
	if err != nil {
		db.DFatalf("unixPair %v\n", err)
	}

	cmd.Stdout = f
	cmd.Stderr = f
	cmd.ExtraFiles = []*os.File{rdr}

	ino := ino(rdr)

	// Set up new namespaces
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS,
	}
	db.DPrintf(db.CONTAINER, "exec cmd %v", cmd)

	s := time.Now()
	if err := cmd.Start(); err != nil {
		db.DPrintf(db.CONTAINER, "Error start %v %v", cmd, err)
		CleanupUproc(uproc.GetPid())
		return nil, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Uproc cmd.Start %v", uproc.GetPid(), time.Since(s))

	rdr.Close()

	return &uprocCmd{cmd: cmd, ino: ino}, nil
}

func CleanupUproc(pid sp.Tpid) {
	if err := os.RemoveAll(jailPath(pid)); err != nil {
		db.DPrintf(db.ALWAYS, "Error cleanupJail: %v", err)
	}
}

func jailPath(pid sp.Tpid) string {
	return filepath.Join(sp.SIGMAHOME, "jail", pid.String())
}

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

	//fileinfo, _ := os.Stat(sp.SIGMA_NETPROXY_SOCKET)
	//stat, _ := fileinfo.Sys().(*syscall.Stat_t)

	verbose := db.IsLabelSet(db.CRIU)
	root := "/home/sigmaos/jail/" + spid.String() + "/"
	extino := "unix[" + strconv.FormatInt(int64(ino), 10) + "]"
	db.DPrintf(db.ALWAYS, "stat %v\n", extino)
	opts := &rpc.CriuOpts{
		Pid:         proto.Int32(int32(pid)),
		ImagesDirFd: proto.Int32(int32(img.Fd())),
		Root:        proto.String(root),
		//TcpEstablished: proto.Bool(true),
		TcpClose:     proto.Bool(true), // XXX does it matter on dump?
		External:     []string{extino, "unix[/tmp/spproxyd/spproxyd-netproxy.sock]", "mnt[/lib]:libMount", "mnt[/lib64]:lib64Mount", "mnt[/usr]:usrMount", "mnt[/etc]:etcMount", "mnt[/bin]:binMount", "mnt[/dev]:devMount", "mnt[/tmp]:tmpMount", "mnt[/tmp/sigmaos-perf]:perfMount", "mnt[/mnt]:mntMount", "mnt[/mnt/binfs]:binfsMount"},
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

func RestoreProc(criuInst *criu.Criu, proc *proc.Proc, imgDir, pages string) error {
	db.DPrintf(db.CKPT, "RestoreProc %v %v %v", proc, imgDir, pages)
	if err := restoreMounts(proc.GetPid()); err != nil {
		return err
	}
	jailPath := "/home/sigmaos/jail/" + proc.GetPid().String() + "/"
	if LAZY {
		err := runLazypagesd(imgDir, pages)
		db.DPrintf(db.CKPT, "lazyPages err %v", err)
		if err != nil {
			return err
		}
	}
	return restoreProc(criuInst, proc, imgDir, jailPath)
}

func runLazypagesd(imgDir, pages string) error {
	db.DPrintf(db.CKPT, "Start lazypagesd img %v pages %v", imgDir, pages)
	cmd := exec.Command("lazypagesd", []string{imgDir, pages}...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		db.DPrintf(db.CKPT, "read pipe err %v\n", err)
		return err
	}

	go func() {
		db.DPrintf(db.CKPT, "Wait lazypagesd %v %v", imgDir, pages)
		err := cmd.Wait()
		db.DPrintf(db.CKPT, "Wait lazypagesd returns %v", err)
	}()
	return nil
}

func restoreProc(criuInst *criu.Criu, proc *proc.Proc, imgDir, jailPath string) error {
	db.DPrintf(db.CKPT, "restoreProc %v", imgDir)
	img, err := os.Open(imgDir)
	if err != nil {
		db.DPrintf(db.CKPT, "restoreProc: Open %v err", imgDir, err)
		return err
	}
	defer img.Close()

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

	// XXX read inventory
	criuDump, err := lazypagessrv.ReadImg(imgDir, "2", "fdinfo")
	if err != nil {
		db.DFatalf("ReadImg fdinfo err %v\n", err)
	}
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

	db.DPrintf(db.ALWAYS, "Invoke restore with fd %d dstfd %v key %v\n", fd, dstfd, inostr)

	opts := &rpc.CriuOpts{
		ImagesDirFd: proto.Int32(int32(img.Fd())),
		Root:        proto.String(jailPath),
		// TcpEstablished: proto.Bool(true),
		TcpClose: proto.Bool(true),
		External: []string{"unix[/tmp/spproxyd/spproxyd-netproxy.sock]", "mnt[libMount]:/lib", "mnt[lib64Mount]:/lib64", "mnt[usrMount]:/usr", "mnt[etcMount]:/etc", "mnt[binMount]:/home/sigmaos/bin/user", "mnt[devMount]:/dev", "mnt[tmpMount]:/tmp", "mnt[perfMount]:/tmp/sigmaos-perf", "mnt[mntMount]:/mnt", "mnt[binfsMount]:/mnt/binfs"},
		// Unprivileged: proto.Bool(true),
		LazyPages: proto.Bool(LAZY),
		InheritFd: ifds,
	}
	if verbose {
		opts.LogLevel = proto.Int32(4)
		opts.LogFile = proto.String("restore.log")
	}

	go func() {
		err = criuInst.Restore(opts, nil)
		db.DPrintf(db.CKPT, "restoreProc: Restore err %v", err)
		if verbose {
			dumpLog(imgDir + "/restore.log")
		}
	}()

	// wait for restored proc to check in
	b = make([]byte, 1)
	if _, err := wrt.Read(b); err != nil {
		db.DFatalf("sendConn err %v\n", err)
	}

	db.DPrintf(db.ALWAYS, "restored proc is running")

	if err := sendConn(wrt, uconn); err != nil {
		db.DFatalf("sendConn err %v\n", err)
	}

	if err := sendProcEnv(wrt, proc); err != nil {
		db.DFatalf("sendConn err %v\n", err)
	}

	db.DPrintf(db.ALWAYS, "sendConn: sent")

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
	db.DPrintf(db.ALWAYS, "fd %v ino %d\n", fd, st.Ino)
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
