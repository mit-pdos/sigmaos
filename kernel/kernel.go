package kernel

import (
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

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

type Kernel struct {
	sync.Mutex
	*fslib.FsLib
	*procclnt.ProcClnt
	realmId     string
	namedAddr   []string
	procdIp     string
	cores       *sessp.Tinterval
	fss3d       []*Subsystem
	fsuxd       []*Subsystem
	procd       []*Subsystem
	dbd         []*Subsystem
	nameds      []*Subsystem // if > 1, then running with replicated named
	crashedPids map[proc.Tpid]bool
}

func mkKernel(realmId string, namedAddr []string, cores *sessp.Tinterval) *Kernel {
	k := &Kernel{}
	k.realmId = realmId
	k.namedAddr = namedAddr
	k.cores = cores
	k.procd = []*Subsystem{}
	k.fsuxd = []*Subsystem{}
	k.fss3d = []*Subsystem{}
	k.dbd = []*Subsystem{}
	k.nameds = []*Subsystem{}
	k.crashedPids = make(map[proc.Tpid]bool)
	return k
}

func (k *Kernel) addNamed(nd *Subsystem) {
	k.Lock()
	defer k.Unlock()
	k.nameds = append(k.nameds, nd)
}

func MakeKernel(realm string, p *Param) (*Kernel, error) {
	cores := sessp.MkInterval(0, uint64(linuxsched.NCores))
	k := mkKernel(realm, fslib.Named(), cores)
	if p.Services[0] == sp.NAMEDREL {
		k.makeNameds(p)
		p.Services = p.Services[1:]
	}
	proc.SetProgram(p.Uname)
	proc.SetPid(proc.GenPid())
	fsl, err := fslib.MakeFsLibAddr(p.Uname, fslib.Named())
	if err != nil {
		return nil, err
	}
	k.FsLib = fsl
	startSrvs(k, p)
	return k, err
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
func (k *Kernel) makeNameds(p *Param) error {
	n := len(fslib.Named())
	ch := make(chan error)
	k.startNameds(ch, n, p)
	var err error
	for i := 0; i < n; i++ {
		r := <-ch
		if r != nil {
			err = r
		}
	}
	return err
}

func (k *Kernel) startNameds(ch chan error, n int, p *Param) {
	for i := 0; i < n; i++ {
		// Must happen in a separate thread because MakeKernelNamed
		// will block until the replicas are able to process requests.
		go func(i int) {
			err := bootNamed(k, p.Uname, i)
			ch <- err
		}(i)
	}
}

// replicaId is used to index into the fslib.Named() slice and select
// an address for this named.
func bootNamed(k *Kernel, uname string, replicaId int) error {
	// replicaId needs to be 1-indexed for replication library.
	cmd, err := RunNamed(fslib.Named()[replicaId], len(fslib.Named()) > 1, replicaId+1, fslib.Named(), NO_REALM)
	if err != nil {
		return err
	}
	ss := makeSubsystemCmd(nil, nil, "", false, cmd)
	k.addNamed(ss)
	time.Sleep(SLEEP_MS * time.Millisecond)
	return err
}

// Start kernel services listed in p
func startSrvs(k *Kernel, p *Param) error {
	// XXX should this be GetPid?
	if len(p.Services) == 0 {
		return nil
	}
	k.ProcClnt = procclnt.MakeProcClntInit(proc.GenPid(), k.FsLib, p.Uname, k.namedAddr)
	err := k.BootSubs()
	if err != nil {
		db.DPrintf(db.KERNEL, "Start err %v\n", err)
		return err
	}
	return nil
}

// Boot subsystems other than named
func (k *Kernel) BootSubs() error {
	// Procd must boot first, since other services are spawned as
	// procs.
	if err := k.bootProcd(true); err != nil {
		return err
	}
	if err := k.BootFsUxd(); err != nil {
		return err
	}
	if err := k.BootFss3d(); err != nil {
		return err
	}
	if err := k.BootDbd(); err != nil {
		return err
	}
	return nil
}

func (k *Kernel) bootSubsystem(binpath string, args []string, procdIp string, viaProcd bool, list *[]*Subsystem) error {
	k.Lock()
	defer k.Unlock()

	pid := proc.Tpid(path.Base(binpath) + "-" + proc.GenPid().String())
	p := proc.MakeProcPid(pid, binpath, args)
	ss := makeSubsystem(k.ProcClnt, p, procdIp, viaProcd)
	// Lock appending to list
	*list = append(*list, ss)
	return ss.Run(k.namedAddr)
}

func (k *Kernel) BootProcd() error {
	return k.bootProcd(false)
}

// Boot a procd. If spawningSys is true, procd will wait for all kernel procs
// to be spawned before claiming any procs.
func (k *Kernel) bootProcd(spawningSys bool) error {
	err := k.bootSubsystem("kernel/procd", []string{path.Join(k.realmId, "bin"), k.cores.Marshal(), strconv.FormatBool(spawningSys)}, "", false, &k.procd)
	if err != nil {
		return err
	}
	if k.procdIp == "" {
		k.procdIp = k.GetProcdIp()
	}
	return nil
}

func (k *Kernel) BootFsUxd() error {
	return k.bootSubsystem("kernel/fsuxd", []string{path.Join(sp.UXROOT, k.realmId)}, k.procdIp, true, &k.fsuxd)
}

func (k *Kernel) BootFss3d() error {
	return k.bootSubsystem("kernel/fss3d", []string{k.realmId}, k.procdIp, true, &k.fss3d)
}

func (k *Kernel) BootDbd() error {
	var dbdaddr string
	dbdaddr = os.Getenv("SIGMADBADDR")
	// XXX don't pass dbd addr as an envvar, it's messy.
	if dbdaddr == "" {
		dbdaddr = "127.0.0.1:3306"
	}
	return k.bootSubsystem("kernel/dbd", []string{dbdaddr}, k.procdIp, true, &k.dbd)
	return nil
}

func (k *Kernel) GetProcdIp() string {
	k.Lock()
	defer k.Unlock()

	if len(k.procd) != 1 {
		db.DFatalf("Error unexpexted num procds: %v", k.procd)
	}
	return GetSubsystemInfo(k.FsLib, sp.KPIDS, k.procd[0].p.Pid.String()).Ip
}

func (k *Kernel) KillOne(srv string) error {
	k.Lock()
	defer k.Unlock()

	var err error
	var ss *Subsystem
	switch srv {
	case sp.PROCD:
		if len(k.procd) > 0 {
			ss = k.procd[0]
			k.procd = k.procd[1:]
		} else {
			db.DPrintf(db.ALWAYS, "Tried to kill procd, nothing to kill")
		}
	case sp.UX:
		if len(k.fsuxd) > 0 {
			ss = k.fsuxd[0]
			k.fsuxd = k.fsuxd[1:]
		} else {
			db.DPrintf(db.ALWAYS, "Tried to kill ux, nothing to kill")
		}
	default:
		db.DFatalf("Unkown server type in Kernel.KillOne: %v", srv)
	}
	err = ss.Kill()
	if err == nil {
		ss.Wait()
		k.crashedPids[ss.p.Pid] = true
	} else {
		db.DFatalf("%v kill failed %v\n", srv, err)
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
			if _, ok := k.crashedPids[pid]; !ok {
				if status, err := k.WaitExit(pid); err != nil || !status.IsStatusEvicted() {
					db.DPrintf(db.ALWAYS, "shutdown error pid %v: %v %v", pid, status, err)
				}
			}
			db.DPrintf(db.KERNEL, "Done evicting %v", pid)
		}
	}
	// Make sure the procs actually exited
	for _, d := range k.fss3d {
		d.Wait()
	}
	for _, d := range k.fsuxd {
		d.Wait()
	}
	for _, d := range k.procd {
		d.Wait()
	}
	for _, d := range k.dbd {
		d.Wait()
	}
	for _, d := range k.nameds {
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

	return proc.MakeProcPid(proc.Tpid("pid-"+strconv.Itoa(id)+proc.GenPid().String()), "kernel/named", args)
}

// Run a named (but not as a proc)
func RunNamed(addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, error) {
	p := makeNamedProc(addr, replicate, id, peers, realmId)
	cmd, err := kproc.RunKernelProc(p, fslib.Named(), false)
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
	s := mkKernel(realmId, namedAddr, cores)
	fsl, err := fslib.MakeFsLibAddr(uname, namedAddr)
	if err != nil {
		return nil, err
	}
	s.FsLib = fsl
	s.ProcClnt = procclnt.MakeProcClntInit(proc.GenPid(), s.FsLib, uname, namedAddr)
	return s, nil
}

// Run a named as a proc
func BootNamed(pclnt *procclnt.ProcClnt, addr string, replicate bool, id int, peers []string, realmId string) (*exec.Cmd, proc.Tpid, error) {
	p := makeNamedProc(addr, replicate, id, peers, realmId)
	cmd, err := pclnt.SpawnKernelProc(p, fslib.Named(), "", false)
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
