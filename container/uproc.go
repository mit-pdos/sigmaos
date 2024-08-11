package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	criu "github.com/checkpoint-restore/go-criu/v7"
	"github.com/checkpoint-restore/go-criu/v7/rpc"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/binsrv"
)

const IMGDIR = "/home/sigmaos/ckptimg/"

type uprocCmd struct {
	cmd *exec.Cmd
}

func (upc *uprocCmd) Wait() error {
	return upc.cmd.Wait()
}

func (upc *uprocCmd) Pid() int {
	return upc.cmd.Process.Pid
}

// Contain user procs using exec-uproc-rs trampoline
func StartUProc(uproc *proc.Proc, netproxy bool) (*uprocCmd, error) {
	db.DPrintf(db.CONTAINER, "RunUProc netproxy %v %v env %v\n", netproxy, uproc, os.Environ())
	var cmd *exec.Cmd
	straceProcs := proc.GetLabels(uproc.GetProcEnv().GetStrace())

	pn := binsrv.BinPath(uproc.GetVersionedProgram())
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

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

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
	return &uprocCmd{cmd: cmd}, nil
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

func CheckpointProc(c *criu.Criu, pid int, spid sp.Tpid) (string, error) {

	// defer cleanupJail(pid)

	db.DPrintf(db.ALWAYS, "CheckpointProc %v", pid)

	procImgDir := IMGDIR + fmt.Sprint(pid)
	err := os.MkdirAll(procImgDir, os.ModePerm)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Checkpointing: error creating img dir %v", err)
		return procImgDir, err
	}
	img, err := os.Open(procImgDir)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Checkpointing: error opening img dir %v", err)
		return procImgDir, err
	}
	defer img.Close()

	root := "/home/sigmaos/jail/" + spid.String() + "/"
	opts := &rpc.CriuOpts{}
	// TODO might need to manually add all of these external mounts to the checkpoint?
	// TODO or at the least, since the FS is not checkpointed, and /local stuff persisted across sigmaos, keep that?
	db.DPrintf(db.ALWAYS, "opts do not include perf")
	opts = &rpc.CriuOpts{
		Pid:            proto.Int32(int32(pid)),
		ImagesDirFd:    proto.Int32(int32(img.Fd())),
		LogLevel:       proto.Int32(4),
		TcpEstablished: proto.Bool(true),
		Root:           proto.String(root),
		SkipMnt:        []string{"/mnt/binfs"},
		External:       []string{"mnt[/lib]:libMount", "mnt[/lib64]:lib64Mount", "mnt[/usr]:usrMount", "mnt[/etc]:etcMount", "mnt[/bin]:binMount", "mnt[/dev]:devMount", "mnt[/tmp/sigmaos-perf]:perfMount", "mnt[/mnt]:mntMount", "mnt[/tmp]:tmpMount", "mnt[/home/sigmaos/bin/user]ubinMount"},
		//Unprivileged:   proto.Bool(true),
		//ShellJob: proto.Bool(true),
		// ExtUnixSk: proto.Bool(true),   // for datagram sockets but for streaming
		LogFile: proto.String("dump.log"),
	}

	db.DPrintf(db.ALWAYS, "starting checkpoint")
	err = c.Dump(opts, NoNotify{})
	db.DPrintf(db.ALWAYS, "finished checkpoint")
	if err != nil {
		db.DPrintf(db.ALWAYS, "Checkpointing: Dumping failed %v", err)
		b, err0 := os.ReadFile(procImgDir + "/dump.log")
		if err0 != nil {
			db.DPrintf(db.ALWAYS, "Checkpointing: opening dump.log failed %v", err0)
		}
		db.DPrintf(db.ALWAYS, "Checkpointing: Dumping failed %s", string(b))
		return procImgDir, err
	} else {
		db.DPrintf(db.ALWAYS, "Checkpointing: Dumping succeeded!")
	}

	return procImgDir, nil
}

func restoreMounts(sigmaPid string) error {
	// create dir for proc to be put in
	jailPath := "/home/sigmaos/jail/" + sigmaPid + "/"
	os.Mkdir(jailPath, 0777)

	// redo mounts
	// Mount /lib
	dstMount := jailPath + "lib"
	os.Mkdir(dstMount, 0755)
	if err := syscall.Mount("/lib", dstMount, "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /lib: %v", err)
		return err
	}
	// Mount /lib64
	dstMount = jailPath + "lib64"
	os.Mkdir(dstMount, 0755)
	if err := syscall.Mount("/lib64", dstMount, "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /lib64: %v", err)
		return err
	}
	// Mount /proc
	dstMount = jailPath + "proc"
	os.Mkdir(dstMount, 0755)
	if err := syscall.Mount(dstMount, dstMount, "proc", 0, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /proc: %v", err)
		return err
	}
	// Mount realm's user bin directory as /bin
	dstMount = jailPath + "bin"
	os.Mkdir(dstMount, 0755)
	if err := syscall.Mount(filepath.Join(sp.SIGMAHOME, "bin/user"), dstMount, "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount userbin: %v", err)
		return err
	}
	// Mount /usr
	dstMount = jailPath + "usr"
	os.Mkdir(dstMount, 0755)
	if err := syscall.Mount("/usr", dstMount, "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /usr: %v", err)
		return err
	}
	// Mount /dev/urandom
	dstMount = jailPath + "dev"
	os.Mkdir(dstMount, 0755)
	if err := syscall.Mount("/dev", dstMount, "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /dev: %v", err)
		return err
	}
	// Mount /etc
	dstMount = jailPath + "etc"
	os.Mkdir(dstMount, 0755)
	if err := syscall.Mount("/etc", dstMount, "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /etc: %v", err)
		return err
	}

	// Mount /tmp
	dstMount = jailPath + "tmp"
	os.Mkdir(dstMount, 0755)
	if err := syscall.Mount("/tmp", dstMount, "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /tmp: %v", err)
		return err
	}

	// Mount perf dir (remove starting first slash)
	dstMount = jailPath + "tmp/sigmaos-perf"
	os.Mkdir(jailPath+"tmp", 0755)
	os.Mkdir(dstMount, 0755)
	if err := syscall.Mount("/tmp/sigmaos-perf", jailPath+"tmp/sigmaos-perf", "none", syscall.MS_BIND, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount perfoutput: %v", err)
		return err
	}

	// Mount /tmp
	dstMount = jailPath + "mnt"
	os.Mkdir(dstMount, 0755)
	if err := syscall.Mount("/mnt", dstMount, "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /mnt: %v", err)
		return err
	}

	db.DPrintf(db.ALWAYS, "done making mounts!")
	return nil
}

func RestoreRunProc(criuInst *criu.Criu, sigmaPid string, osPid int) error {
	imgDir := IMGDIR + fmt.Sprint(osPid)

	if err := restoreMounts(sigmaPid); err != nil {
		return err
	}
	jailPath := "/home/sigmaos/jail/" + sigmaPid + "/"
	return restoreProc(criuInst, imgDir, jailPath)

	// signalling finish is done via sigmaos
	// TODO potentially need to wait for another checkpoint signal
}

func restoreProc(criuInst *criu.Criu, localChkptLoc, jailPath string) error {
	// open img dir
	img, err := os.Open(localChkptLoc)
	if err != nil {
		db.DPrintf(db.ALWAYS, "can't open image dir:", err)
		return err
	}
	defer img.Close()

	// TODO how do I make this dependent on whether we're using perf? Will that need to be part of the uproc state? Kinda annoying...
	opts := &rpc.CriuOpts{
		ImagesDirFd:    proto.Int32(int32(img.Fd())),
		LogLevel:       proto.Int32(4),
		TcpEstablished: proto.Bool(true),
		Root:           proto.String(jailPath),
		External:       []string{"mnt[libMount]:/lib", "mnt[lib64Mount]:/lib64", "mnt[usrMount]:/usr", "mnt[etcMount]:/etc", "mnt[/bin]:binMount", "mnt[devMount]:/dev", "mnt[/tmp/sigmaos-perf]:perfMount", "mnt[/mnt]:mntMount", "mnt[/tmp]:tmpMount", "mnt[ubinMount]:/home/sigmaos/bin/user"},
		// Unprivileged:   proto.Bool(true),
		LogFile: proto.String("restore.log"),
	}

	db.DPrintf(db.ALWAYS, "just before restoring")

	err = criuInst.Restore(opts, nil)

	if err != nil {
		db.DPrintf(db.ALWAYS, "Restoring: Restoring failed %v %s", err, err.Error())
		b, err := os.ReadFile(localChkptLoc + "/restore.log")
		if err != nil {
			db.DPrintf(db.ALWAYS, "Restoring: opening restore.log failed %v", err)
		}
		str := string(b)
		db.DPrintf(db.ALWAYS, "Restoring: Restoring failed %s", str)
		return err
	} else {
		db.DPrintf(db.ALWAYS, "Restoring: Restoring suceeded!")
	}

	return nil

	// signalling finish is done via sigmaos
	// TODO potentially need to wait for another checkpoint signal
}
