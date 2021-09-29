package realm

import (
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ulambda/fslib"
	"ulambda/named"
	"ulambda/sync"
)

const (
	N_REPLICAS = "N_REPLICAS"
)

const (
	SLEEP_MS = 100
)

func BootNamedReplicas(fsl *fslib.FsLib, bin string, addrs []string, realmId string) ([]*exec.Cmd, error) {
	cmds := []*exec.Cmd{}
	for i, addr := range addrs {
		cmd, err := BootNamed(fsl, bin, addr, len(addrs) > 1, i+1, addrs, realmId)
		if err != nil {
			log.Fatalf("Error BootNamed in BootAllNameds: %v", err)
			return nil, err
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

// Boot a named and set up the initfs
func BootNamed(rootFsl *fslib.FsLib, bin string, addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, error) {
	var args []string
	if realmId == NO_REALM {
		args = []string{"0", addr, NO_REALM}
	} else {
		args = []string{"0", addr, realmId}
	}
	// If we're running replicated...
	if replicate {
		args = append(args, strconv.Itoa(id))
		args = append(args, strings.Join(peers[:id], ","))
	}

	// If this isn't the root named, create a cond to wait on
	var namedStartCond *sync.Cond
	if rootFsl != nil {
		namedStartCond = sync.MakeCond(rootFsl, path.Join(named.BOOT, addr), nil)
		namedStartCond.Init()
	}

	cmd, err := run(bin, "/bin/kernel/named", fslib.Named(), args)
	if err != nil {
		log.Printf("Error running named: %v", err)
		return nil, err
	}

	if rootFsl != nil {
		namedStartCond.Wait()
	} else {
		time.Sleep(SLEEP_MS * time.Millisecond)
	}

	fsl := fslib.MakeFsLibAddr("realm", []string{addr})
	if err := named.MakeInitFs(fsl); err != nil && !strings.Contains(err.Error(), "Name exists") {
		log.Printf("MakeInitFs error: %v", err)
		return nil, err
	}
	return cmd, nil
}

func ShutdownNamedReplicas(addrs []string) {
	for _, addr := range addrs {
		ShutdownNamed(addr)
	}
}

func ShutdownNamed(namedAddr string) {
	fsl := fslib.MakeFsLibAddr("realm", []string{namedAddr})
	// Shutdown named last
	err := fsl.Remove(named.NAMED + "/")
	if err != nil {
		// XXX sometimes we get EOF..
		if err.Error() == "EOF" {
			log.Printf("Remove %v shutdown %v\n", named.NAMED, err)
		} else {
			log.Fatalf("Remove %v shutdown %v\n", named.NAMED, err)
		}
	}
}

func run(bin string, name string, namedAddr []string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(path.Join(bin, name), args...)
	// Create a process group ID to kill all children if necessary.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ())
	cmd.Env = append(cmd.Env, "NAMED="+strings.Join(namedAddr, ","))
	return cmd, cmd.Start()
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
