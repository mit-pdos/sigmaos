package kernel

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/kproc"
	"sigmaos/port"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	SLEEP_S          = 2
	REPL_PORT_OFFSET = 100
	SUBSYSTEM_INFO   = "subsystem-info"

	FPORT port.Tport = 1112
	LPORT port.Tport = 1132
)

type Param struct {
	KernelId string
	Services []string
	Dbip     string
	Mongoip  string
	Jaegerip string
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
	if p.Services[0] == sp.KNAMED {
		k.namedAddr = nameds
		if err := k.bootKNamed(true); err != nil {
			return nil, err
		}
		p.Services = p.Services[1:]
	}
	proc.SetSigmaNamed(k.namedAddr)
	proc.SetSigmaJaegerIP(p.Jaegerip)
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
	if len(k.svcs.svcs[sp.KNAMED]) > 0 && len(k.svcs.svcs[sp.NAMEDREL]) > 0 {
		// a kernel with knamed and named; stop knamed
		if err := k.KillOne(sp.KNAMED); err != nil {
			db.DPrintf(db.KERNEL, "MakeKernel: stop knamed err %v\n", err)
			return nil, err
		}
		db.DPrintf(db.KERNEL, "MakeKernel: switch to named\n")
	}
	return k, err
}

func (k *Kernel) Ip() string {
	return k.ip
}

func (k *Kernel) Shutdown() error {
	db.DPrintf(db.KERNEL, "Shutdown %v\n", k.Param.KernelId)
	k.shutdown()
	N := 200 // Crashing procds in mr test leave several fids open; maybe too many?
	n := k.PathClnt.FidClnt.Len()
	if n > N {
		log.Printf("Too many FIDs open (%v): %v", n, k.PathClnt.FidClnt)
	}
	db.DPrintf(db.KERNEL, "Shutdown %s done\n", k.Param.KernelId)
	return nil
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
	// start knamed to shutdown kernel with named?
	if len(k.svcs.svcs[sp.KNAMED]) == 0 && len(k.svcs.svcs[sp.NAMEDREL]) > 0 {
		if err := k.bootKNamed(false); err != nil {
			db.DFatalf("shutdown: bootKnamed err %v\n", err)
		}
	}
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
			db.DPrintf(db.KERNEL, "RemoveChild %v %v", pid, proc.GetProcDir())
			if err := k.RemoveChild(pid); err != nil {
				db.DPrintf(db.KERNEL, "Done evicting; rm %v err %v", pid, err)
			} else {
				db.DPrintf(db.KERNEL, "Done evicting; rm %v", pid)
			}

			db.DPrintf(db.KERNEL, "Done evicting %v", pid)
		}
	}
	for key, val := range k.svcs.svcs {
		if key != sp.KNAMED {
			for _, ss := range val {
				ss.Wait()
			}
		}
	}
	if err := k.RmDir(proc.GetProcDir()); err != nil {
		db.DPrintf(db.KERNEL, "Failed to clean up %v err %v", proc.GetProcDir(), err)
	}
	db.DPrintf(db.KERNEL, "Shutdown nameds %d\n", len(k.svcs.svcs[sp.KNAMED]))
	for _, ss := range k.svcs.svcs[sp.KNAMED] {
		// kill it so that test terminates;  XXX should this be  d.Terminate()?
		db.DPrintf(db.KERNEL, "Kill %v %v\n", sp.KNAMED, ss)
		ss.Kill()
		// d.Wait()
	}
}

func makeKNamedProc(realmId sp.Trealm, init bool) (*proc.Proc, error) {
	i := "start"
	if init {
		i = "init"
	}
	args := []string{realmId.String(), i}
	p := proc.MakePrivProcPid(proc.Tpid("pid-"+proc.GenPid().String()), "knamed", args, true)
	return p, nil
}

// Run knamed (but not as a proc)
func runKNamed(p *proc.Proc, addr sp.Taddrs, realmId sp.Trealm, init bool) (*exec.Cmd, error) {
	r1, w1, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	r2, w2, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd, err := kproc.RunKernelProc(p, addr, realmId, []*os.File{w1, r2, w2})
	if err != nil {
		r1.Close()
		w1.Close()
		r2.Close()
		w2.Close()
		db.DPrintf(db.ALWAYS, "RunKernelProc named: %v", err)
		return nil, err
	}
	w1.Close()
	r2.Close()
	defer r1.Close()

	data, err := ioutil.ReadAll(r1)
	if err != nil {
		db.DPrintf(db.ALWAYS, "pipe read err %v", err)
		return nil, err
	}
	db.DPrintf(db.ALWAYS, "knamed reports %v\n", string(data))
	return cmd, nil
}

func stopKNamed(cmd *exec.Cmd) error {
	db.DPrintf(db.KERNEL, "stopKNamed %v\n", cmd)
	w2 := cmd.ExtraFiles[2]
	_, err := fmt.Fprintf(w2, "stop")
	w2.Close()
	return err
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
