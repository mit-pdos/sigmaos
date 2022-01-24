package realm

import (
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"

	"ulambda/fslib"
	"ulambda/kernel"
	np "ulambda/ninep"
	"ulambda/procclnt"
)

const (
	N_REPLICAS = "N_REPLICAS"
)

func BootNamedReplicas(bin string, addrs []string, realmId string) ([]*exec.Cmd, error) {
	cmds := []*exec.Cmd{}
	for i, addr := range addrs {
		cmd, err := kernel.RunNamed(bin, addr, len(addrs) > 1, i+1, addrs, realmId)
		if err != nil {
			log.Fatalf("Error BootNamed in BootAllNameds: %v", err)
			return nil, err
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

func ShutdownNamedReplicas(pclnt *procclnt.ProcClnt, pids []string) {
	for _, pid := range pids {
		if err := pclnt.EvictKernelProc(pid); err != nil {
			log.Fatalf("Error Evict in Realm.ShutdownNamedReplicas: %v", err)
		}
	}
}

func ShutdownNamed(namedAddr string) {
	fsl := fslib.MakeFsLibAddr("realm", []string{namedAddr})
	// Shutdown named last
	err := fsl.ShutdownFs(np.NAMED)
	if err != nil {
		// XXX sometimes we get EOF..
		if err.Error() == "EOF" {
			log.Printf("Remove %v shutdown %v\n", np.NAMED, err)
		} else {
			log.Fatalf("Remove %v shutdown %v\n", np.NAMED, err)
		}
	}
}

// Generate an address for a new named
func genNamedAddrs(n int, localIP string) []string {
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
			log.Fatalf("Invalid N_REPLICAS format: %v", err)
		}
		return n
	}
	return 1
}
