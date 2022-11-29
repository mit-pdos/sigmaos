package machine

import (
	"os"
	"os/exec"
	"path"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/linuxsched"
	"sigmaos/memfssrv"
	"sigmaos/namespace"
	np "sigmaos/ninep"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/resource"
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
	memfssrv *memfssrv.MemFs
}

func MakeMachined(args []string) *Machined {
	m := &Machined{}
	m.nodeds = map[proc.Tpid]*exec.Cmd{}
	m.Config = makeMachineConfig()
	m.FsLib = fslib.MakeFsLib(proc.GetPid().String())
	m.ProcClnt = procclnt.MakeProcClntInit(proc.GetPid(), m.FsLib, proc.GetPid().String(), fslib.Named())
	mfs, err := memfssrv.MakeMemFsFsl(MACHINES, m.FsLib, m.ProcClnt)
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}
	m.memfssrv = mfs
	m.path = path.Join(MACHINES, m.memfssrv.MyAddr())
	resource.MakeCtlFile(m.receiveResourceGrant, m.handleResourceRequest, m.memfssrv.Root(), np.RESOURCE_CTL)
	m.initFS()
	m.memfssrv.GetStats().MonitorCPUUtil(nil)
	m.cleanLinuxFS()
	return m
}

// Remove old files from previous runs.
func (m *Machined) cleanLinuxFS() {
	os.Mkdir(namespace.NAMESPACE_DIR, 0777)
	sts, err := os.ReadDir(namespace.NAMESPACE_DIR)
	if err != nil {
		db.DFatalf("Error ReadDir: %v", err)
	}
	for _, st := range sts {
		if err := os.RemoveAll(path.Join(namespace.NAMESPACE_DIR, st.Name())); err != nil {
			db.DFatalf("Error RemoveAll: %v", err)
		}
	}
}

func (m *Machined) receiveResourceGrant(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Tnode:
		m.shutdownNoded(proc.Tpid(msg.Name))
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

func (m *Machined) handleResourceRequest(msg *resource.ResourceMsg) {
	switch msg.ResourceType {
	case resource.Tnode:
		db.DPrintf("MACHINED", "Request to boot noded with name %v", msg.Name)
		m.bootNoded(proc.Tpid(msg.Name))
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

// Boot a fresh noded.
func (m *Machined) bootNoded(pid proc.Tpid) {
	m.Lock()
	defer m.Unlock()

	db.DPrintf("MACHINED", "Booting noded %v", pid)

	p := proc.MakeProcPid(pid, "realm/noded", []string{m.memfssrv.MyAddr()})
	noded, err := m.SpawnKernelProc(p, fslib.Named(), "", false)
	if err != nil {
		db.DFatalf("RunKernelProc: %v", err)
	}
	m.nodeds[pid] = noded
	if err := m.WaitStart(pid); err != nil {
		db.DFatalf("Error WaitStart(%v): %v", pid, err)
	}

	db.DPrintf("MACHINED", "Finished booting noded %v", pid)
}

// Shut down a noded.
func (m *Machined) shutdownNoded(pid proc.Tpid) {
	m.Lock()
	defer m.Unlock()

	if err := m.Evict(pid); err != nil {
		db.DFatalf("Error evict: %v", err)
	}
	if status, err := m.WaitExit(pid); err != nil || !status.IsStatusEvicted() {
		db.DFatalf("Error WaitExit: s %v e %v", status, err)
	}
	delete(m.nodeds, pid)
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
	coreGroupSize := uint64(np.Conf.Machine.CORE_GROUP_FRACTION * float64(linuxsched.NCores))
	for i := uint64(0); i < uint64(linuxsched.NCores); i += coreGroupSize {
		iv := np.MkInterval(i, i+coreGroupSize)
		if uint(iv.End) > linuxsched.NCores+1 {
			iv.End = uint64(linuxsched.NCores + 1)
		}
		PostCores(m.FsLib, m.memfssrv.MyAddr(), iv)
	}
}

func (m *Machined) postConfig() {
	// Post config in local fs.
	if err := m.PutFileJson(path.Join(MACHINES, m.memfssrv.MyAddr(), CONFIG), 0777, m.Config); err != nil {
		db.DFatalf("Error PutFile: %v", err)
	}
}

func (m *Machined) Work() {
	m.postConfig()
	m.postCores()
	done := make(chan bool)
	<-done
}
