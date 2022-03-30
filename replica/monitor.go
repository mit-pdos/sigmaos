package replica

import (
	"sort"
	"strings"

	"ulambda/atomic"
	db "ulambda/debug"
	"ulambda/fslib"
	//	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

type ReplicaMonitor struct {
	pid          proc.Tpid
	configPath   string
	unionDirPath string
	//	configLock   *sync.Lock
	*fslib.FsLib
	*procclnt.ProcClnt
}

func MakeReplicaMonitor(args []string) *ReplicaMonitor {
	m := &ReplicaMonitor{}
	// Set up paths
	m.pid = proc.Tpid(args[0])
	m.configPath = args[1]
	m.unionDirPath = args[2]
	// Set up fslib
	fsl := fslib.MakeFsLib("memfs-replica-monitor")
	m.FsLib = fsl
	//	m.configLock = sync.MakeLock(fsl, np.LOCKS, m.configPath, true)
	m.ProcClnt = procclnt.MakeProcClnt(fsl)
	db.DPrintf("RMTR", "MakeReplicaMonitor %v", args)
	return m
}

func (m *ReplicaMonitor) updateConfig() {
	replicas, err := m.GetDir(m.unionDirPath)
	if err != nil {
		db.DFatalf("Error reading union dir in monitor: %v", err)
	}
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i].Name < replicas[j].Name
	})
	new := ""
	for _, r := range replicas {
		new += r.Name + "\n"
	}
	m.Remove(m.configPath)
	err = atomic.PutFileAtomic(m.FsLib, m.configPath, 0777, []byte(strings.TrimSpace(new)))
	if err != nil {
		db.DFatalf("Error writing new config file: %v", err)
	}
}

func (m *ReplicaMonitor) Work() {
	m.Started()
	// Get exclusive access to the config file.
	//	if ok := m.configLock.TryLock(); ok {
	m.updateConfig()
	//	m.configLock.Unlock()
	//	}
}

func (m *ReplicaMonitor) Exit() {
	m.Exited(proc.MakeStatus(proc.StatusOK))
}
