package realm

import (
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"time"

	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/procclnt"
)

const (
	N_REPLICAS = "N_REPLICAS"
)

func BootNamedReplicas(addrs []string, realmId string) ([]*exec.Cmd, error) {
	cmds := []*exec.Cmd{}
	for i, addr := range addrs {
		cmd, err := kernel.RunNamed(addr, len(addrs) > 1, i+1, addrs, realmId)
		if err != nil {
			db.DFatalf("Error BootNamed in BootAllNameds: %v", err)
			return nil, err
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

func ShutdownNamedReplicas(pclnt *procclnt.ProcClnt, pids []string) {
	for _, pid := range pids {
		if err := pclnt.EvictKernelProc(pid); err != nil {
			db.DFatalf("Error Evict in Realm.ShutdownNamedReplicas: %v", err)
		}
	}
}

// Generate an address for a new named
func genNamedAddrs(n int, localIP string) []string {
	// Seed to ensure different port numbers are generated.
	rand.Seed(time.Now().UnixNano())
	basePort := MIN_PORT + rand.Intn(MAX_PORT-MIN_PORT)
	addrs := []string{}
	for i := 0; i < n; i++ {
		portStr := strconv.Itoa(basePort + i)
		addr := localIP + ":" + portStr
		addrs = append(addrs, addr)
	}
	return addrs
}

func nReplicas() int {
	if nStr, ok := os.LookupEnv(N_REPLICAS); ok {
		n, err := strconv.Atoi(nStr)
		if err != nil {
			db.DFatalf("Invalid N_REPLICAS format: %v", err)
		}
		return n
	}
	return 1
}
