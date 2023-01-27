package machine

import (
	"os"
	"os/exec"
	"path"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/linuxsched"
	"sigmaos/machine/proto"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

const (
	MACHINES  = "name/machines/"
	CORES     = "cores"
	CONFIG    = "config"
	ALL_CORES = "all-cores"
	NODEDS    = "nodeds"
)

// Machined registers initial machine reseources and starts Nodeds on-demand.
type Machined struct {
	sync.Mutex
	path   string
	nodeds map[proc.Tpid]*exec.Cmd
	*Config
	*procclnt.ProcClnt
	*fslib.FsLib
	pds *protdevsrv.ProtDevSrv
	pdc *protdevclnt.ProtDevClnt
}

func MakeMachined(args []string) *Machined {
	m := &Machined{}
	m.nodeds = map[proc.Tpid]*exec.Cmd{}
	m.Config = makeMachineConfig()
	fsl, err := fslib.MakeFsLib(proc.GetPid().String())
	if err != nil {
		db.DFatalf("Error MakeFsLib: %v", err)
	}
	m.FsLib = fsl
	m.ProcClnt = procclnt.MakeProcClntInit(proc.GetPid(), m.FsLib, proc.GetPid().String())
	mfs, err := memfssrv.MakeMemFsClnt(MACHINES, m.FsLib, m.ProcClnt)
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}
	m.pds, err = protdevsrv.MakeProtDevSrvMemFs(mfs, m)
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}
	m.pdc, err = protdevclnt.MkProtDevClnt(m.pds.FsLib(), sp.SIGMAMGR)
	if err != nil {
		db.DFatalf("Error MkProtDevClnt: %v", err)
	}

	m.path = path.Join(MACHINES, m.pds.MyAddr())
	m.initFS()
	//	m.memfssrv.GetStats().MonitorCPUUtil(nil)
	m.cleanLinuxFS()
	return m
}

const (
	NAMESPACE_DIR = sp.SIGMAHOME + "/isolation"
)

// Remove old files from previous runs.
func (m *Machined) cleanLinuxFS() {
	os.Mkdir(NAMESPACE_DIR, 0777)
	sts, err := os.ReadDir(NAMESPACE_DIR)
	if err != nil {
		db.DFatalf("Error ReadDir: %v", err)
	}
	for _, st := range sts {
		if err := os.RemoveAll(path.Join(NAMESPACE_DIR, st.Name())); err != nil {
			db.DFatalf("Error RemoveAll: %v", err)
		}
	}
}

// Boot a fresh noded.
func (m *Machined) BootNoded(req proto.MachineRequest, res *proto.MachineResponse) error {
	m.Lock()
	defer m.Unlock()

	pid := proc.Tpid(req.NodedId)
	db.DPrintf(db.MACHINED, "Booting noded %v", pid)

	p := proc.MakeProcPid(pid, "realm/noded", []string{m.pds.MyAddr()})
	// XXX need realm name...
	noded, err := m.SpawnKernelProc(p, procclnt.HLINUX)
	if err != nil {
		db.DFatalf("RunKernelProc: %v", err)
	}
	m.nodeds[pid] = noded
	if err := m.WaitStart(pid); err != nil {
		db.DFatalf("Error WaitStart(%v): %v", pid, err)
	}

	db.DPrintf(db.MACHINED, "Finished booting noded %v", pid)
	res.OK = true
	return nil
}

// Shut down a noded.
func (m *Machined) ShutdownNoded(req proto.MachineRequest, res *proto.MachineResponse) error {
	m.Lock()
	defer m.Unlock()

	pid := proc.Tpid(req.NodedId)

	if err := m.Evict(pid); err != nil {
		db.DFatalf("Error evict: %v", err)
	}
	if status, err := m.WaitExit(pid); err != nil || !status.IsStatusEvicted() {
		db.DFatalf("Error WaitExit: s %v e %v", status, err)
	}
	delete(m.nodeds, pid)
	res.OK = true
	return nil
}

func (m *Machined) initFS() {
	dirs := []string{CORES, NODEDS}
	for _, d := range dirs {
		if err := m.MkDir(path.Join(m.path, d), 0777); err != nil {
			db.DFatalf("Error Mkdir: %v", err)
		}
	}
}

func (m *Machined) postCores() {
	coreGroupSize := NodedNCores()
	if coreGroupSize == 0 {
		db.DFatalf("Core group size 0")
	}
	for i := uint64(0); i < uint64(linuxsched.NCores); i += coreGroupSize {
		iv := sessp.MkInterval(i, i+coreGroupSize)
		if uint(iv.End) > linuxsched.NCores+1 {
			iv.End = uint64(linuxsched.NCores + 1)
		}
		PostCores(m.pdc, m.pds.MyAddr(), iv)
	}
}

func (m *Machined) postConfig() {
	// Post config in local fs.
	if err := m.PutFileJson(path.Join(MACHINES, m.pds.MyAddr(), CONFIG), 0777, m.Config); err != nil {
		db.DFatalf("Error PutFile: %v", err)
	}
}

func (m *Machined) Work() {
	m.postConfig()
	m.postCores()
	done := make(chan bool)
	<-done
}
