package machine

import (
	"os/exec"
	"path"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/resource"
)

const (
	MACHINES = "name/machines/"
	CORES    = "cores"
)

// Machined registers initial machine reseources and starts Nodeds on-demand.
type Machined struct {
	sync.Mutex
	path       string
	coresAvail *np.Tinterval
	nodeds     map[proc.Tpid]*exec.Cmd
	*procclnt.ProcClnt
	*fslib.FsLib
	*fslibsrv.MemFs
}

func MakeMachined(args []string) *Machined {
	m := &Machined{}
	linuxsched.ScanTopology()
	m.coresAvail = np.MkInterval(0, np.Toffset(linuxsched.NCores)+1)
	m.nodeds = map[proc.Tpid]*exec.Cmd{}
	m.FsLib = fslib.MakeFsLib(proc.GetPid().String())
	m.ProcClnt = procclnt.MakeProcClntInit(proc.GetPid(), m.FsLib, proc.GetPid().String(), fslib.Named())
	mfs, err := fslibsrv.MakeMemFsFsl(MACHINES, m.FsLib, m.ProcClnt)
	if err != nil {
		db.DFatalf("Error MakeMemFs: %v", err)
	}
	m.MemFs = mfs
	m.path = path.Join(MACHINES, m.MyAddr())
	resource.MakeCtlFile(m.receiveResourceGrant, m.handleResourceRequest, m.Root(), np.RESOURCE_CTL)
	m.makeFS()
	return m
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
		db.DPrintf(db.ALWAYS, "Request to boot noded with name %v", msg.Name)
		m.bootNoded(proc.Tpid(msg.Name))
	default:
		db.DFatalf("Unexpected resource type: %v", msg.ResourceType)
	}
}

// Boot a fresh noded.
func (m *Machined) bootNoded(pid proc.Tpid) {
	m.Lock()
	defer m.Unlock()

	db.DPrintf(db.ALWAYS, "Booting noded %v", pid)

	p := proc.MakeProcPid(pid, "realm/noded", []string{m.MyAddr()})
	noded, err := m.SpawnKernelProc(p, fslib.Named())
	if err != nil {
		db.DFatalf("RunKernelProc: %v", err)
	}
	m.nodeds[pid] = noded
	if err := m.WaitStart(pid); err != nil {
		db.DFatalf("Error WaitStart(%v): %v", pid, err)
	}

	db.DPrintf(db.ALWAYS, "Finished booting noded %v", pid)
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

func (m *Machined) makeFS() {
	dirs := []string{CORES}
	for _, d := range dirs {
		if err := m.MkDir(path.Join(m.path, d), 0777); err != nil {
			db.DFatalf("Error Mkdir: %v", err)
		}
	}
}

// Post chunks of available cores.
func (m *Machined) postCores(s string) {
	// Post cores in local fs.
	if _, err := m.PutFile(path.Join(m.path, CORES, s), 0777, np.OWRITE, []byte(m.coresAvail.String())); err != nil {
		db.DFatalf("Error PutFile: %v", err)
	}
	msg := resource.MakeResourceMsg(resource.Tgrant, resource.Tcore, m.coresAvail.String(), 1)
	if _, err := m.SetFile(np.SIGMACTL, msg.Marshal(), np.OWRITE, 0); err != nil {
		db.DFatalf("Error SetFile in markFree: %v", err)
	}
}

func (m *Machined) Work() {
	m.postCores("0")
	// XXX double-post cores for now for testing purposes.
	m.postCores("1")
	done := make(chan bool)
	<-done
}
