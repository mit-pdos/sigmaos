package kernel

import (
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kproc"
	"sigmaos/linuxsched"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

const (
	NO_PID           = "no-realm"
	NO_REALM         = "no-realm"
	SLEEP_MS         = 200
	REPL_PORT_OFFSET = 100
	SUBSYSTEM_INFO   = "subsystem-info"
)

type Param struct {
	Realm    string   `yalm:"realm, omitempty"`
	Hostip   string   `yalm:"ip, omitempty"`
	Services []string `yalm:"services"`
}

type Kernel struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	Param     *Param
	namedAddr []string
	procdIp   string
	cores     *sessp.Tinterval
	svcs      *Services
	ip        string
}

func mkKernel(param *Param, namedAddr []string, cores *sessp.Tinterval) *Kernel {
	k := &Kernel{}
	k.Param = param
	k.namedAddr = namedAddr
	k.cores = cores
	k.svcs = mkServices()
	return k
}

func MakeKernel(p *Param) (*Kernel, error) {
	cores := sessp.MkInterval(0, uint64(linuxsched.NCores))
	k := mkKernel(p, fslib.Named(), cores)
	proc.SetProgram(os.Args[0])
	proc.SetPid(proc.GenPid())
	proc.SetRealm(p.Realm)
	if p.Services[0] == sp.NAMEDREL {
		k.makeNameds()
		p.Services = p.Services[1:]
	}
	ip, err := container.LocalIP()
	if err != nil {
		return nil, err
	}
	k.ip = ip
	fsl, err := fslib.MakeFsLibAddr(p.Realm, ip, fslib.Named())
	if err != nil {
		return nil, err
	}
	k.FsLib = fsl
	startSrvs(k)
	return k, err
}

func (k *Kernel) Ip() string {
	return k.ip
}

func (k *Kernel) ShutDown() error {
	db.DPrintf(db.KERNEL, "ShutDown\n")
	k.Shutdown()
	N := 200 // Crashing procds in mr test leave several fids open; maybe too many?
	n := k.PathClnt.FidClnt.Len()
	if n > N {
		log.Printf("Too many FIDs open (%v): %v", n, k.PathClnt.FidClnt)
	}
	db.DPrintf(db.KERNEL, "ShutDown done\n")
	return nil
}

// Start nameds and wait until they have started
func (k *Kernel) makeNameds() error {
	n := len(fslib.Named())
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
			err := bootNamed(k, k.Param.Realm, i, k.Param.Realm)
			ch <- err
		}(i)
	}
}

// Start kernel services listed in p
func startSrvs(k *Kernel) error {
	// XXX should this be GetPid?
	k.ProcClnt = procclnt.MakeProcClntInit(proc.GenPid(), k.FsLib, os.Args[0], k.namedAddr)
	n := len(k.Param.Services)
	for _, s := range k.Param.Services {
		err := k.BootSub(s, k.Param, n > 1) // XXX kernel should wait instead of procd?
		if err != nil {
			db.DPrintf(db.KERNEL, "Start %s err %v\n", k.Param, err)
			return err
		}
	}
	return nil
}

func (k *Kernel) Shutdown() {
	if k.ProcClnt != nil {
		cpids, err := k.GetChildren()
		if err != nil {
			db.DFatalf("GetChildren in Kernel.Shutdown: %v", err)
		}
		db.DPrintf(db.KERNEL, "Shutdown children %v", cpids)
		for _, pid := range cpids {
			k.Evict(pid)
			db.DPrintf(db.KERNEL, "Evicted %v", pid)
			if _, ok := k.svcs.crashedPids[pid]; !ok {
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

func makeNamedProc(addr string, replicate bool, id int, pe []string, realmId string) *proc.Proc {
	args := []string{addr, realmId}
	// If we're running replicated...
	if replicate {
		// Add an offset to the peers' port addresses.
		peers := []string{}
		for _, peer := range pe {
			peers = append(peers, addReplPortOffset(peer))
		}
		args = append(args, strconv.Itoa(id))
		args = append(args, strings.Join(peers, ","))
	}

	p := proc.MakePrivProcPid(proc.Tpid("pid-"+strconv.Itoa(id)+proc.GenPid().String()), "named", args, true)
	return p
}

// Run a named (but not as a proc)
func RunNamed(addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, error) {
	p := makeNamedProc(addr, replicate, id, peers, realmId)
	cmd, err := kproc.RunKernelProc(p, fslib.Named(), realmId, false)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error running named: %v", err)
		return nil, err
	}
	time.Sleep(SLEEP_MS * time.Millisecond)
	return cmd, nil
}

func addReplPortOffset(peerAddr string) string {
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

	return host + ":" + newPort
}

//
// XXX kill backward-compatability, but keep for now for noded.go.
//

func MakeSystem(uname, realmId string, namedAddr []string, cores *sessp.Tinterval) (*Kernel, error) {
	p := &Param{Realm: realmId}
	s := mkKernel(p, namedAddr, cores)
	fsl, err := fslib.MakeFsLibAddr(p.Realm, s.ip, namedAddr)
	if err != nil {
		return nil, err
	}
	s.FsLib = fsl
	s.ProcClnt = procclnt.MakeProcClntInit(proc.GenPid(), s.FsLib, p.Realm, namedAddr)
	return s, nil
}

// Run a named as a proc
func BootNamed(pclnt *procclnt.ProcClnt, addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, proc.Tpid, error) {
	p := makeNamedProc(addr, replicate, id, peers, realmId)
	cmd, err := pclnt.SpawnKernelProc(p, fslib.Named(), "", realmId, false)
	if err != nil {
		db.DFatalf("Error SpawnKernelProc BootNamed: %v", err)
		return nil, "", err
	}
	if err = pclnt.WaitStart(p.Pid); err != nil {
		db.DFatalf("Error WaitStart in BootNamed: %v", err)
		return nil, "", err
	}
	return cmd, p.Pid, nil
}

// Boot subsystems other than named
func (k *Kernel) BootSubs() error {
	// Procd must boot first, since other services are spawned as
	// procs.
	for _, s := range []string{sp.PROCDREL, sp.S3REL, sp.UXREL, sp.DBREL} {
		err := k.BootSub(s, nil, true)
		if err != nil {
			return err
		}
	}
	return nil
}
