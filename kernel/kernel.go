package kernel

import (
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/kproc"
	"sigmaos/port"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	SLEEP_MS         = 200
	REPL_PORT_OFFSET = 100
	SUBSYSTEM_INFO   = "subsystem-info"

	FPORT port.Tport = 1112
	LPORT port.Tport = 1132
)

type Param struct {
	KernelId string
	Services []string
	Dbip     string
	Overlays bool
}

type Kernel struct {
	*sigmaclnt.SigmaClnt
	Param     *Param
	namedAddr sp.Taddrs
	svcs      *Services
	ip        string
}

func mkKernel(param *Param, namedAddr sp.Taddrs) *Kernel {
	k := &Kernel{}
	k.Param = param
	k.namedAddr = namedAddr
	k.svcs = mkServices()
	return k
}

func MakeKernel(p *Param, nameds sp.Taddrs) (*Kernel, error) {
	k := mkKernel(p, nameds)
	proc.SetProgram(os.Args[0])
	proc.SetPid(proc.GenPid())
	ip, err := container.LocalIP()
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.KERNEL, "MakeKernel ip %v", ip)
	k.ip = ip
	proc.SetSigmaLocal(ip)
	if p.Services[0] == sp.NAMEDREL {
		k.makeNameds()
		nameds, err := SetNamedIP(k.ip, k.namedAddr)
		log.Printf("NAMEDS: %v\n", nameds)
		if err != nil {
			return nil, err
		}
		k.namedAddr = nameds
		p.Services = p.Services[1:]
	}
	proc.SetSigmaNamed(k.namedAddr)
	sc, err := sigmaclnt.MkSigmaClntRootInit("kernel", ip, k.namedAddr)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error MkSigmaClntProc (%v): %v", k.namedAddr, err)
		return nil, err
	}
	k.SigmaClnt = sc
	err = startSrvs(k)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error startSrvs %v", err)
		return nil, err
	}
	return k, err
}

func (k *Kernel) Ip() string {
	return k.ip
}

func (k *Kernel) Shutdown() error {
	db.DPrintf(db.KERNEL, "ShutDown\n")
	k.shutdown()
	N := 200 // Crashing procds in mr test leave several fids open; maybe too many?
	n := k.PathClnt.FidClnt.Len()
	if n > N {
		log.Printf("Too many FIDs open (%v): %v", n, k.PathClnt.FidClnt)
	}
	db.DPrintf(db.KERNEL, "ShutDown %s done\n", k.Param.KernelId)
	return nil
}

// Start nameds and wait until they have started
func (k *Kernel) makeNameds() error {
	n := len(k.namedAddr)
	ch := make(chan error)
	k.startNameds(ch, n)
	var err error
	for i := 0; i < n; i++ {
		r := <-ch
		if r != nil {
			err = r
		}
	}
	return err
}

func (k *Kernel) startNameds(ch chan error, n int) {
	for i := 0; i < n; i++ {
		// Must happen in a separate thread because MakeKernelNamed
		// will block until the replicas are able to process requests.
		go func(i int) {
			err := bootNamed(k, "rootnamed", i, sp.ROOTREALM)
			ch <- err
		}(i)
	}
}

// Start kernel services listed in p
func startSrvs(k *Kernel) error {
	n := len(k.Param.Services)
	for _, s := range k.Param.Services {
		_, err := k.BootSub(s, nil, k.Param, n > 1) // XXX kernel should wait instead of procd?
		if err != nil {
			db.DPrintf(db.KERNEL, "Start %v err %v\n", k.Param, err)
			return err
		}
	}
	return nil
}

func (k *Kernel) shutdown() {
	if len(k.Param.Services) > 0 {
		db.DPrintf(db.KERNEL, "Get children %v", proc.GetPid())
		cpids, err := k.GetChildren()
		if err != nil {
			db.DPrintf(db.KERNEL, "Error get children: %v", err)
			db.DFatalf("GetChildren in Kernel.Shutdown: %v", err)
		}
		db.DPrintf(db.KERNEL, "Shutdown children %v", cpids)
		for _, pid := range cpids {
			k.Evict(pid)
			db.DPrintf(db.KERNEL, "Evicted %v", pid)
			if !k.svcs.svcMap[pid].crashed {
				if status, err := k.WaitExit(pid); err != nil || !status.IsStatusEvicted() {
					db.DPrintf(db.ALWAYS, "shutdown error pid %v: %v %v", pid, status, err)
				}
			}
			db.DPrintf(db.KERNEL, "Done evicting %v", pid)
		}
	}
	for key, val := range k.svcs.svcs {
		if key != sp.NAMEDREL {
			for _, d := range val {
				d.Wait()
			}
		}
	}
	for _, d := range k.svcs.svcs[sp.NAMEDREL] {
		// kill it so that test terminates
		d.Terminate()
		d.Wait()
	}
}

func makeNamedProc(addr *sp.Taddr, replicate bool, id int, pe sp.Taddrs, realmId sp.Trealm) (*proc.Proc, error) {
	args := []string{addr.Addr, realmId.String(), ""}
	// If we're running replicated...
	if replicate {
		// Add an offset to the peers' port addresses.
		peers := sp.Taddrs{}
		for _, peer := range pe {
			peers = append(peers, addReplPortOffset(peer.Addr))
		}
		args = append(args, strconv.Itoa(id))
		s, err := peers.Taddrs2String()
		if err != nil {
			return nil, err
		}
		args = append(args, s)
	}

	p := proc.MakePrivProcPid(proc.Tpid("pid-"+strconv.Itoa(id)+proc.GenPid().String()), "named", args, true)
	return p, nil
}

// Run a named (but not as a proc)
func RunNamed(addr *sp.Taddr, replicate bool, id int, peers sp.Taddrs, realmId sp.Trealm) (*exec.Cmd, error) {
	p, err := makeNamedProc(addr, replicate, id, peers, realmId)
	if err != nil {
		return nil, err
	}
	cmd, err := kproc.RunKernelProc(p, peers, realmId)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error running named: %v", err)
		return nil, err
	}
	time.Sleep(SLEEP_MS * time.Millisecond)
	return cmd, nil
}

func SetNamedIP(ip string, ports sp.Taddrs) (sp.Taddrs, error) {
	nameds := make(sp.Taddrs, len(ports))
	for i, s := range ports {
		host, port, err := net.SplitHostPort(s.Addr)
		if err != nil {
			return nil, err
		}
		if host != "" {
			db.DFatalf("Tried to substitute named ip when port exists: %v -> %v %v", s, host, port)
		}
		nameds[i] = sp.MkTaddr(net.JoinHostPort(ip, port))
	}
	return nameds, nil
}

func addReplPortOffset(peerAddr string) *sp.Taddr {
	// Compute replica address as peerAddr + REPL_PORT_OFFSET
	host, port, err := net.SplitHostPort(peerAddr)
	if err != nil {
		db.DFatalf("Error splitting host port: %v", err)
	}
	portI, err := strconv.Atoi(port)
	if err != nil {
		db.DFatalf("Error conv port: %v", err)
	}
	newPort := strconv.Itoa(portI + REPL_PORT_OFFSET)

	return sp.MkTaddr(host + ":" + newPort)
}
