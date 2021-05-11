package memfsd_replica

import (
	"log"
	"path"
	"sort"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
)

type MemfsReplicaMonitor struct {
	pid          string
	configPath   string
	unionDirPath string
	*fslib.FsLib
}

func MakeMemfsReplicaMonitor(args []string) *MemfsReplicaMonitor {
	m := &MemfsReplicaMonitor{}
	// Set up paths
	m.pid = args[0]
	m.configPath = args[1]
	m.unionDirPath = args[2]
	// Set up fslib
	fsl := fslib.MakeFsLib("memfs-replica-monitor")
	m.FsLib = fsl
	db.DLPrintf("RMTR", "MakeMemfsReplicaMonitor %v", args)
	return m
}

func (m *MemfsReplicaMonitor) updateConfig() {
	replicas, err := m.ReadDir(m.unionDirPath)
	if err != nil {
		log.Fatalf("Error reading union dir in monitor: %v", err)
	}
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i].Name < replicas[j].Name
	})
	new := ""
	for _, r := range replicas {
		new += r.Name + "\n"
	}
	m.Remove(m.configPath)
	err = m.MakeDirFileAtomic(path.Dir(m.configPath), path.Base(m.configPath), []byte(strings.TrimSpace(new)))
	if err != nil {
		log.Fatalf("Error writing new config file: %v", err)
	}
}

func (m *MemfsReplicaMonitor) Work() {
	m.Started(m.pid)
	// Get exclusive access to the config file.
	if ok := m.TryLockFile(fslib.LOCKS, m.configPath); ok {
		m.updateConfig()
		m.UnlockFile(fslib.LOCKS, m.configPath)
	}
}
