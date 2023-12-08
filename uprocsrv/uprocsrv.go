package uprocsrv

import (
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/kernelclnt"
	"sigmaos/perf"
	"sigmaos/port"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/uprocsrv/proto"

	criu "github.com/checkpoint-restore/go-criu/v7"
)

type UprocSrv struct {
	mu       sync.RWMutex
	ch       chan struct{}
	pcfg     *proc.ProcEnv
	ssrv     *sigmasrv.SigmaSrv
	kc       *kernelclnt.KernelClnt
	procs    map[string]chan container.CheckpointSignal
	criuInst *criu.Criu
	kernelId string
	realm    sp.Trealm
	assigned bool
}

func RunUprocSrv(kernelId string, up string) error {
	pcfg := proc.GetProcEnv()
	ups := &UprocSrv{kernelId: kernelId, ch: make(chan struct{}), procs: make(map[string]chan container.CheckpointSignal), criuInst: criu.MakeCriu()}

	db.DPrintf(db.UPROCD, "Run %v %v %s IP %s", kernelId, up, os.Environ(), pcfg.GetLocalIP())

	var ssrv *sigmasrv.SigmaSrv
	var err error
	if up == port.NOPORT.String() {
		pn := path.Join(sp.SCHEDD, kernelId, sp.UPROCDREL, pcfg.GetPID().String())
		ssrv, err = sigmasrv.NewSigmaSrv(pn, ups, pcfg)
	} else {
		// The kernel will advertise the server, so pass "" as pn.
		ssrv, err = sigmasrv.NewSigmaSrvPort("", up, pcfg, ups)
	}
	if err != nil {
		return err
	}
	if err := shrinkMountTable(); err != nil {
		db.DFatalf("Error shrinking mount table: %v", err)
	}
	ups.ssrv = ssrv
	p, err := perf.NewPerf(pcfg, perf.UPROCD)
	if err != nil {
		db.DFatalf("Error NewPerf: %v", err)
	}
	defer p.Done()
	err = ssrv.RunServer()
	db.DPrintf(db.UPROCD, "RunServer done %v\n", err)
	return nil
}

func shrinkMountTable() error {
	return nil
	mounts := []string{
		"/etc/resolv.conf",
		"/etc/hostname",
		"/etc/hosts",
		"/dev/shm",
		"/dev/mqueue",
		"/dev/pts",
		//		"/dev",
	}
	for _, mnt := range mounts {
		b := append([]byte(mnt), 0)
		_, _, errno := unix.Syscall(unix.SYS_UMOUNT2, uintptr(unsafe.Pointer(&b[0])), uintptr(0), uintptr(0))
		if errno != 0 {
			db.DFatalf("Error umount2 %v: %v", mnt, errno)
			return errno
		}
	}
	lazyUmounts := []string{
		"/sys",
	}
	for _, mnt := range lazyUmounts {
		b := append([]byte(mnt), 0)
		_, _, errno := unix.Syscall(unix.SYS_UMOUNT2, uintptr(unsafe.Pointer(&b[0])), uintptr(unix.MNT_DETACH), uintptr(0))
		if errno != 0 {
			db.DFatalf("Error umount2 %v: %v", mnt, errno)
			return errno
		}
	}
	return nil
}

func (ups *UprocSrv) assignToRealm(realm sp.Trealm) error {
	ups.mu.RLock()
	defer ups.mu.RUnlock()

	// If already assigned, bail out
	if ups.assigned {
		return nil
	}

	// Promote lock
	ups.mu.RUnlock()
	ups.mu.Lock()
	// If already assigned, demote lock & bail out
	if ups.assigned {
		ups.mu.Unlock()
		ups.mu.RLock()
		return nil
	}

	db.DPrintf(db.UPROCD, "Assign Uprocd to realm %v", realm)
	err := container.MountRealmBinDir(realm)
	if err != nil {
		db.DFatalf("Error mount realm bin dir: %v", err)
	}

	db.DPrintf(db.UPROCD, "Assign Uprocd to realm %v done", realm)
	// Note that the uprocsrv has been assigned.
	ups.assigned = true

	// Demote to reader lock
	ups.mu.Unlock()
	ups.mu.RLock()

	return err
}

func (ups *UprocSrv) Assign(ctx fs.CtxI, req proto.AssignRequest, res *proto.AssignResult) error {
	// no-op
	res.OK = true
	return nil
}

func (ups *UprocSrv) Run(ctx fs.CtxI, req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.NewProcFromProto(req.ProcProto)
	db.DPrintf(db.UPROCD, "Run uproc %v", uproc)
	// Assign this uprocsrv to the realm, if not already assigned.
	if err := ups.assignToRealm(uproc.GetRealm()); err != nil {
		db.DFatalf("Err assign to realm: %v", err)
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Uproc Run: %v", uproc.GetPid(), time.Since(uproc.GetSpawnTime()))

	procChan := make(chan container.CheckpointSignal)
	ups.procs[uproc.ProcEnvProto.PidStr] = procChan

	// if proc is restore, do something different
	if uproc.ProcEnvProto.CheckpointLocation != "" {
		db.DPrintf(db.ALWAYS, "restoring proc")
		err := os.MkdirAll("/home/sigmaos/chkptimg", 0777)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error creating local chkpt dir: %s", err)
		}
		localChkptLoc := "/home/sigmaos/chkptimg/" + uproc.ProcEnvProto.PidStr
		ups.readCheckpointFromS3(uproc.ProcEnvProto.CheckpointLocation, localChkptLoc)
		return container.RestoreRunProc(ups.criuInst, localChkptLoc, uproc.ProcEnvProto.PidStr, int(uproc.ProcEnvProto.OsPid))
	} else {
		db.DPrintf(db.ALWAYS, "running proc as normal")
		return container.RunUProc(uproc, procChan)
	}
}

func (ups *UprocSrv) writeCheckpointToS3(chkptLocalDir string, chkptSimgaDir string) error {

	ups.ssrv.MemFs.SigmaClnt().MkDir(chkptSimgaDir, 0777)
	db.DPrintf(db.UPROCD, "created dir: %v\n", chkptSimgaDir)

	// loop through files in curr dir, put into files in sigmaOS dir
	files, err := os.ReadDir(chkptLocalDir)
	if err != nil {
		db.DPrintf(db.UPROCD, "Error reading local checkpoint directory on uprocd: %v\n", err)
		return err
	}

	// TODO make this just use CopyFile?
	for _, file := range files {
		db.DPrintf(db.UPROCD, "Trying to copy file %s\n", file.Name())
		dstFd, err := ups.ssrv.MemFs.SigmaClnt().Create(filepath.Join(chkptSimgaDir, file.Name()), 0777, sp.OWRITE)
		if err != nil {
			db.DPrintf(db.UPROCD, "Error creating file %s: %v\n", file.Name(), err)
			return err
		}

		// write content
		fileContents, err := os.ReadFile(filepath.Join(chkptLocalDir, file.Name()))
		if err != nil {
			db.DPrintf(db.UPROCD, "Error reading file: %v\n", err)
			return err
		}
		_, err = ups.ssrv.MemFs.SigmaClnt().Write(dstFd, fileContents)
		if err != nil {
			db.DPrintf(db.UPROCD, "Error writing to sigmaos file: %v\n", err)
			return err
		}

		// close
		err = ups.ssrv.MemFs.SigmaClnt().Close(dstFd)
		if err != nil {
			db.DPrintf(db.UPROCD, "Error closing sigmaos file: %v\n", err)
			return err
		}
	}
	return nil
}

func (ups *UprocSrv) readCheckpointFromS3(remoteImgDir string, localImgDir string) {

	dunno, err := ups.ssrv.MemFs.SigmaClnt().GetDir(remoteImgDir)
	if err != nil {
		db.DFatalf("Error reading remote checkpoint directory on uprocd: %v\n", err.Error())
	}
	files := sp.Names(dunno)

	err = os.Mkdir(localImgDir, 0777)
	if err != nil {
		db.DFatalf("Error creating local chkptdir: %v\n", err)
	}

	for _, file := range files {
		db.DPrintf("Trying to copy file %s\n", file)

		// write content
		fileContents, _ := ups.ssrv.MemFs.SigmaClnt().GetFile(filepath.Join(remoteImgDir, file))
		if err != nil {
			db.DFatalf("Error reading remote file: %v\n", err)
		}
		err = os.WriteFile(filepath.Join(localImgDir, file), fileContents, 0777)
		if err != nil {
			db.DFatalf("Error writing to local file: %v\n", err)
		}
	}

}

func (ups *UprocSrv) Checkpoint(ctx fs.CtxI, req proto.CheckpointPidRequest, res *proto.CheckpointPidResult) error {
	db.DPrintf(db.UPROCD, "Checkpointing uproc %v", req.PidStr)
	procChan := ups.procs[req.PidStr]

	// sts, _ := ups.ssrv.MemFs.SigmaClnt().GetDir(sp.S3)
	// currS3Info := sp.Names(sts)[0]

	ups.ssrv.MemFs.SigmaClnt().MkDir(sp.S3+"~any/sigmaoscheckpoint/", 0777)
	db.DPrintf(db.UPROCD, "created dir: %v\n", sp.S3+"~any/sigmaoscheckpoint/")

	chkptSimgaDir := sp.S3 + "~any/sigmaoscheckpoint/" + req.PidStr + "/"
	chkptLocalDir, osPid, err := container.CheckpointProc(ups.criuInst, procChan)

	res.OsPid = int32(osPid)
	res.CheckpointLocation = chkptSimgaDir

	if err != nil {
		return err
	}
	err = ups.writeCheckpointToS3(chkptLocalDir, chkptSimgaDir)
	if err != nil {
		// TODO clean up what was written partially?
		return err
	}
	// close chan?
	return nil
}
