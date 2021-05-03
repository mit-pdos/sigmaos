package memfsd_replica

import (
	"log"
	"path"
	"sort"
	"strings"

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
	log.Printf("MakeMemfsReplicaMonitor %v", args)
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
	for {
		done := make(chan bool)
		m.SetDirWatch(m.unionDirPath, func(p string, err error) {
			log.Printf("Dir watch triggered!")
			if err != nil && err.Error() == "EOF" {
				return
			} else if err != nil {
				log.Printf("Error in ReplicaMonitor DirWatch: %v", err)
			}
			done <- true
		})
		<-done
		m.updateConfig()
	}
	m.Exiting(m.pid, "OK")
}
