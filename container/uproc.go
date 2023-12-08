package container

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"
	"time"

	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/proc"
	sp "sigmaos/sigmap"

	criu "github.com/checkpoint-restore/go-criu/v7"
	"github.com/checkpoint-restore/go-criu/v7/rpc"
	"google.golang.org/protobuf/proto"
)

type CheckpointSignal struct {
	RealPid    int
	SimgaosPid sp.Tpid
	usingPerf  bool
}

//
// Contain user procs using exec-uproc-rs trampoline
//

func RunUProc(uproc *proc.Proc, procChan chan CheckpointSignal) error {
	db.DPrintf(db.CONTAINER, "RunUProc %v env %v\n", uproc, os.Environ())
	var cmd *exec.Cmd
	// straceProcs := proc.GetLabels(uproc.GetProcEnv().GetStrace())
	// Optionally strace the proc
	// if straceProcs[uproc.GetProgram()] {
	// 	cmd = exec.Command("strace", append([]string{"-f", "exec-uproc-rs", uproc.GetPid().String(), uproc.GetProgram()}, uproc.Args...)...)
	// } else {
	cmd = exec.Command("exec-uproc-rs", append([]string{uproc.GetPid().String(), uproc.GetProgram()}, uproc.Args...)...)
	// }
	// cmd = exec.Command(uproc.GetProgram(), uproc.Args...)
	uproc.AppendEnv("PATH", "/bin:/bin2:/usr/bin:/home/sigmaos/bin/kernel:/bin/user")
	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))
	// uproc.AppendEnv(proc.SIGMAPERF, uproc.GetProcEnv().GetPerf())
	//	uproc.AppendEnv("RUST_BACKTRACE", "1")
	cmd.Env = uproc.GetEnv()
	db.DPrintf(db.ALWAYS, "env: %v", uproc.GetEnv())
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
	db.DPrintf(db.CONTAINER, "exec %v\n", cmd)

	// defer cleanupJail(uproc.GetPid())
	s := time.Now()
	if err := cmd.Start(); err != nil {
		db.DPrintf(db.CONTAINER, "Error start %v %v", cmd, err)
		return err
	}
	db.DPrintf(db.ALWAYS, "---> RUNNING WITH PID %d\n", cmd.Process.Pid)
	db.DPrintf(db.SPAWN_LAT, "[%v] Uproc cmd.Start %v", uproc.GetPid(), time.Since(s))
	if uproc.GetType() == proc.T_BE {
		s := time.Now()
		setSchedPolicy(cmd.Process.Pid, linuxsched.SCHED_IDLE)
		db.DPrintf(db.SPAWN_LAT, "[%v] Uproc Get/Set sched attr %v", uproc.GetPid(), time.Since(s))
	}

	procDone := make(chan error)

	go func() {
		err := cmd.Wait()
		procDone <- err
	}()

	// waits for proc to finish or signal to checkpoint
	select {
	case <-procChan:
		db.DPrintf(db.ALWAYS, "got checkpoint signal, sending pid and exiting")
		// TODO this is not working?
		// usingPerf := uproc.GetProcEnv().GetPerf() != ""
		toSend := CheckpointSignal{
			RealPid:    cmd.Process.Pid,
			SimgaosPid: uproc.GetPid(),
			usingPerf:  true}
		procChan <- toSend
		return nil
	case err := <-procDone:
		db.DPrintf(db.ALWAYS, "proc done")
		if err != nil {
			db.DPrintf(db.ALWAYS, "wait error was %s", err)
			return err
		}
	}

	db.DPrintf(db.CONTAINER, "ExecUProc done  %v\n", uproc)
	return nil
}

func cleanupJail(pid sp.Tpid) {
	if err := os.RemoveAll(jailPath(pid)); err != nil {
		db.DPrintf(db.ALWAYS, "Error cleanupJail: %v", err)
	}
}

func setSchedPolicy(pid int, policy linuxsched.SchedPolicy) {
	attr, err := linuxsched.SchedGetAttr(pid)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error Getattr %v: %v", pid, err)
		return
	}
	attr.Policy = policy
	err = linuxsched.SchedSetAttr(pid, attr)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error Setattr %v: %v", pid, err)
	}
}

func jailPath(pid sp.Tpid) string {
	return path.Join(sp.SIGMAHOME, "jail", pid.String())
}

type NoNotify struct {
	criu.NoNotify
}

func CheckpointProc(c *criu.Criu, procChan chan CheckpointSignal) (string, int, error) {

	imgDir := "/home/sigmaos/chkptimg"

	procChan <- CheckpointSignal{}

	// wait on channel for pid
	select {
	case chkptSignal := <-procChan:

		defer cleanupJail(chkptSignal.SimgaosPid)

		pid := chkptSignal.RealPid
		db.DPrintf(db.ALWAYS, "got pid from channel: %v", pid)

		procImgDir := imgDir + "/" + fmt.Sprint(pid)
		err := os.MkdirAll(procImgDir, os.ModePerm)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Checkpointing: error creating img dir %v", err)
			return procImgDir, pid, err
		}
		img, err := os.Open(procImgDir)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Checkpointing: error opening img dir %v", err)
			return procImgDir, pid, err
		}
		defer img.Close()

		root := "/home/sigmaos/jail/" + chkptSignal.SimgaosPid.String() + "/"
		opts := &rpc.CriuOpts{}
		// TODO might need to manually add all of these external mounts to the checkpoint?
		// TODO or at the least, since the FS is not checkpointed, and /local stuff persisted across sigmaos, keep that?
		if chkptSignal.usingPerf {
			db.DPrintf(db.ALWAYS, "opts include perf")
			opts = &rpc.CriuOpts{
				Pid:            proto.Int32(int32(pid)),
				ImagesDirFd:    proto.Int32(int32(img.Fd())),
				LogLevel:       proto.Int32(4),
				TcpEstablished: proto.Bool(true),
				Root:           proto.String(root),
				External:       []string{"mnt[/lib]:libMount", "mnt[/lib64]:lib64Mount", "mnt[/usr]:usrMount", "mnt[/etc]:etcMount", "mnt[/bin]:binMount", "mnt[/dev]:devMount", "mnt[/tmp/sigmaos-perf]:perfMount"},
				Unprivileged:   proto.Bool(true),
				ShellJob:       proto.Bool(true),
				LogFile:        proto.String("dump.log"),
			}
		} else {
			db.DPrintf(db.ALWAYS, "opts do not include perf")
			opts = &rpc.CriuOpts{
				Pid:            proto.Int32(int32(pid)),
				ImagesDirFd:    proto.Int32(int32(img.Fd())),
				LogLevel:       proto.Int32(4),
				TcpEstablished: proto.Bool(true),
				Root:           proto.String(root),
				External:       []string{"mnt[/lib]:libMount", "mnt[/lib64]:lib64Mount", "mnt[/usr]:usrMount", "mnt[/etc]:etcMount", "mnt[/bin]:binMount", "mnt[/dev]:devMount"},
				Unprivileged:   proto.Bool(true),
				ShellJob:       proto.Bool(true),
				LogFile:        proto.String("dump.log"),
			}
		}
		// opts = &rpc.CriuOpts{
		// 	Pid:          proto.Int32(int32(pid)),
		// 	ImagesDirFd:  proto.Int32(int32(img.Fd())),
		// 	LogLevel:     proto.Int32(4),
		// 	Unprivileged: proto.Bool(true),
		// 	// TcpEstablished: proto.Bool(true),
		// 	ShellJob: proto.Bool(true),
		// 	LogFile:  proto.String("dump.log"),
		// }

		db.DPrintf(db.ALWAYS, "starting checkpoint")
		err = c.Dump(opts, NoNotify{})
		db.DPrintf(db.ALWAYS, "finished checkpoint")
		if err != nil {
			db.DPrintf(db.ALWAYS, "Checkpointing: Dumping failed %v", err)
			b, err := os.ReadFile(procImgDir + "/dump.log")
			if err != nil {
				db.DPrintf(db.ALWAYS, "Checkpointing: opening dump.log failed %v", err)
			}
			str := string(b)
			db.DPrintf(db.ALWAYS, "Checkpointing: Dumping failed %s", str)
			return procImgDir, pid, err
		} else {
			db.DPrintf(db.ALWAYS, "Checkpointing: Dumping succeeded!")
		}

		return procImgDir, pid, nil

	}
}

func RestoreRunProc(criuInst *criu.Criu, localChkptLoc string, sigmaPid string, osPid int) error {

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
	if err := syscall.Mount(path.Join(sp.SIGMAHOME, "bin/user"), dstMount, "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount userbin: %v", err)
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

	// db.DPrintf(db.ALWAYS, "done making mounts!")

	// open img dir
	img, err := os.Open(localChkptLoc)
	if err != nil {
		db.DPrintf(db.ALWAYS, "can't open image dir:", err)
	}
	defer img.Close()

	// TODO how do I make this dependent on whether we're using perf? Will that need to be part of the uproc state? Kinda annoying...
	opts := &rpc.CriuOpts{
		ImagesDirFd:    proto.Int32(int32(img.Fd())),
		LogLevel:       proto.Int32(4),
		ShellJob:       proto.Bool(true),
		TcpEstablished: proto.Bool(true),
		Root:           proto.String(jailPath),
		External:       []string{"mnt[libMount]:/lib", "mnt[lib64Mount]:/lib64", "mnt[usrMount]:/usr", "mnt[etcMount]:/etc", "mnt[devMount]:/dev", "mnt[perfMount]:/tmp/sigmaos-perf", "mnt[binMount]:/home/sigmaos/bin/user"},
		Unprivileged:   proto.Bool(true),
		LogFile:        proto.String("restore.log"),
	}
	// opts := &rpc.CriuOpts{
	// 	ImagesDirFd: proto.Int32(int32(img.Fd())),
	// 	LogLevel:    proto.Int32(4),
	// 	ShellJob:    proto.Bool(true),
	// 	// TcpEstablished: proto.Bool(true),
	// 	Unprivileged: proto.Bool(true),
	// 	LogFile:      proto.String("restore.log"),
	// }

	db.DPrintf(db.ALWAYS, "just before restoring")

	// err = criuInst.Restore(opts, nil)

	// if err != nil {
	// 	db.DPrintf(db.ALWAYS, "Restoring: Restoring failed %v %s", err, err.Error())
	// 	b, err := os.ReadFile(localChkptLoc + "/restore.log")
	// 	if err != nil {
	// 		db.DPrintf(db.ALWAYS, "Restoring: opening restore.log failed %v", err)
	// 	}
	// 	str := string(b)
	// 	db.DPrintf(db.ALWAYS, "Restoring: Restoring failed %s", str)
	// } else {
	// 	db.DPrintf(db.ALWAYS, "Restoring: Restoring suceeded!")
	// }

	// return nil

	restDone := make(chan error)

	go func() {
		err = criuInst.Restore(opts, nil)
		restDone <- err
	}()

	timer := time.NewTicker(100 * time.Millisecond)

	// waits for proc to finish or signal to checkpoint
	for {
		select {
		case <-restDone:
			db.DPrintf(db.ALWAYS, "done restoring")
			if err != nil {
				db.DPrintf(db.ALWAYS, "Restoring: Restoring failed %v %s", err, err.Error())
				b, err := os.ReadFile(localChkptLoc + "/restore.log")
				if err != nil {
					db.DPrintf(db.ALWAYS, "Restoring: opening restore.log failed %v", err)
				}
				str := string(b)
				db.DPrintf(db.ALWAYS, "Restoring: Restoring failed %s", str)
			} else {
				db.DPrintf(db.ALWAYS, "Restoring: Restoring suceeded!")
				b, err := os.ReadFile(localChkptLoc + "/restore.log")
				if err != nil {
					db.DPrintf(db.ALWAYS, "Restoring: opening restore.log failed %v", err)
				}
				str := string(b)
				db.DPrintf(db.ALWAYS, "Restore.log: %s", str)
			}
		case <-timer.C:
			db.DPrintf(db.ALWAYS, "Restoring: timer went off")
			b, err := os.ReadFile(localChkptLoc + "/restore.log")
			if err != nil {
				db.DPrintf(db.ALWAYS, "Restoring: opening restore.log failed %v", err)
			}
			str := string(b)
			db.DPrintf(db.ALWAYS, "Restoring: Restoring failed %s", str)
			return nil
		}
	}

	// signalling finish is done via sigmaos
	// TODO potentially need to wait for another checkpoint signal
}

//
// The exec-uproc trampoline enters here
//

func ExecUProc() error {
	db.DPrintf(db.CONTAINER, "ExecUProc: %v\n", os.Args)
	args := os.Args[1:]
	program := args[0]
	s := time.Now()
	pcfg := proc.GetProcEnv()
	// Isolate the user proc.
	pn, err := isolateUserProc(pcfg.GetPID(), program)
	db.DPrintf(db.SPAWN_LAT, "[%v] Uproc isolation %v", pcfg.GetPID(), time.Since(s))
	if err != nil {
		return err
	}
	os.Setenv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	db.DPrintf(db.CONTAINER, "exec %v %v", pn, args)
	if err := syscall.Exec(pn, args, os.Environ()); err != nil {
		db.DPrintf(db.CONTAINER, "Error exec %v", err)
		return err
	}
	defer finishIsolation()
	return nil
}

// For debugging
func ls(dir string) error {
	db.DPrintf(db.ALWAYS, "== ls %s\n", dir)
	files, err := os.ReadDir(dir)
	if err != nil {
		db.DPrintf(db.ALWAYS, "ls err %v", err)
		return nil
	}
	for _, file := range files {
		db.DPrintf(db.ALWAYS, "file %v isdir %v", file.Name(), file.IsDir())
	}
	return nil
}
