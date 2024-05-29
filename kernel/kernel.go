package kernel

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/kproc"
	"sigmaos/netsigma"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	MAX_EVICT_RETRIES = 10
)

type Param struct {
	KernelID    string
	Services    []string
	Dbip        string
	Mongoip     string
	Overlays    bool
	NetProxy    bool
	BuildTag    string
	GVisor      bool
	ReserveMcpu string
}

type Kernel struct {
	sync.Mutex
	*sigmaclnt.SigmaClntKernel
	Param        *Param
	realms       map[sp.Trealm]*sigmaclnt.SigmaClntKernel
	svcs         *Services
	ip           sp.Tip
	shuttingDown bool
}

func newKernel(param *Param) *Kernel {
	return &Kernel{
		realms: make(map[sp.Trealm]*sigmaclnt.SigmaClntKernel),
		Param:  param,
		svcs:   newServices(),
	}
}

func NewKernel(p *Param, pe *proc.ProcEnv) (*Kernel, error) {
	k := newKernel(p)
	ip, err := netsigma.LocalIP()
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.KERNEL, "NewKernel ip %v", ip)
	k.ip = ip
	pe.SetInnerContainerIP(ip)
	pe.SetOuterContainerIP(ip)
	if p.Services[0] == sp.KNAMED {
		if err := k.bootKNamed(pe, true); err != nil {
			return nil, err
		}
		p.Services = p.Services[1:]
	}
	sc, err := sigmaclnt.NewSigmaClntRootInit(pe)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error NewSigmaClntProc: %v", err)
		return nil, err
	}
	k.SigmaClntKernel = sigmaclnt.NewSigmaClntKernel(sc)
	db.DPrintf(db.KERNEL, "Kernel start srvs %v", k.Param.Services)
	err = startSrvs(k)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error startSrvs %v", err)
		return nil, err
	}
	if len(k.svcs.svcs[sp.KNAMED]) > 0 && len(k.svcs.svcs[sp.NAMEDREL]) > 0 {
		// a kernel with knamed and named; stop knamed
		if err := k.KillOne(sp.KNAMED); err != nil {
			db.DPrintf(db.KERNEL, "NewKernel: stop knamed err %v\n", err)
			return nil, err
		}
		db.DPrintf(db.KERNEL, "NewKernel: switch to named\n")
	}
	// Eagerly remove kernel's proc dir if this is just a sigmaclntd kernel
	// since it isn't needed (and otherwise will slow down shutdown)
	if k.IsPurelySigmaclntdKernel() {
		if err := k.RmDir(k.ProcEnv().ProcDir); err != nil {
			db.DPrintf(db.KERNEL, "Failed to clean up sigmaclntkernel procdir %v err %v", k.ProcEnv().ProcDir, err)
		}
	}
	return k, err
}

func (k *Kernel) Ip() sp.Tip {
	return k.ip
}

func (k *Kernel) IsPurelySigmaclntdKernel() bool {
	db.DPrintf(db.KERNEL, "Check is sigmaclntd kernel: %v", k.Param.Services)
	return len(k.Param.Services) == 1 && k.Param.Services[0] == sp.SIGMACLNTDREL
}

func (k *Kernel) Shutdown() error {
	k.Lock()
	defer k.Unlock()

	k.shuttingDown = true

	db.DPrintf(db.KERNEL, "Shutdown %v\n", k.Param.KernelID)
	k.shutdown()
	db.DPrintf(db.KERNEL, "Shutdown %s done\n", k.Param.KernelID)
	return nil
}

func (k *Kernel) getRealmSigmaClnt(realm sp.Trealm) (*sigmaclnt.SigmaClntKernel, error) {
	sck, ok := k.realms[realm]
	if ok {
		return sck, nil
	}
	pe := proc.NewDifferentRealmProcEnv(k.ProcEnv(), realm)
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewSigmaClnt: %v", err)
		return nil, err
	}
	sck = sigmaclnt.NewSigmaClntKernel(sc)
	k.realms[realm] = sck
	return sck, nil
}

// Start kernel services
func startSrvs(k *Kernel) error {
	for _, s := range k.Param.Services {
		_, err := k.BootSub(s, nil, k.Param, sp.ROOTREALM)
		if err != nil {
			db.DPrintf(db.KERNEL, "StartSRv %v %v err %v\n", s, k.Param, err)
			return err
		}
	}
	return nil
}

func (k *Kernel) shutdown() {
	// start knamed to shutdown kernel with named?
	if len(k.svcs.svcs[sp.KNAMED]) == 0 && len(k.svcs.svcs[sp.NAMEDREL]) > 0 {
		db.DPrintf(db.KERNEL, "Booting knamed for shutdown %v", k.ProcEnv().GetPID())
		if err := k.bootKNamed(k.ProcEnv(), false); err != nil {
			db.DFatalf("shutdown: bootKnamed err %v\n", err)
		}
		db.DPrintf(db.KERNEL, "Done booting knamed for shutdown %v", k.ProcEnv().GetPID())
	}
	if len(k.Param.Services) > 0 {
		cpids := []sp.Tpid{}
		for pid, _ := range k.svcs.svcMap {
			cpids = append(cpids, pid)
		}
		// Sort schedds to the end, to avoid havingas many eviction errors.
		sort.Slice(cpids, func(i, j int) bool {
			if strings.HasPrefix(cpids[i].String(), "schedd-") {
				if strings.HasPrefix(cpids[j].String(), "schedd-") {
					return strings.Compare(cpids[i].String(), cpids[j].String()) < 0
				}
				return false
			}
			return true
		})
		db.DPrintf(db.KERNEL, "Shutdown children %v", cpids)
		for _, pid := range cpids {
			for i := 0; i < MAX_EVICT_RETRIES; i++ {
				err := k.svcs.svcMap[pid].Evict()
				if err == nil || !serr.IsErrCode(err, serr.TErrUnreachable) {
					db.DPrintf(db.KERNEL, "Evicted proc %v %T err %v", pid, k.svcs.svcMap[pid], err)
					k.svcs.svcMap[pid].Wait()
					break
				}
				if i == MAX_EVICT_RETRIES-1 {
					db.DPrintf(db.KERNEL, "Giving up trying to evict kernel proc! %v", pid)
				}
				db.DPrintf(db.KERNEL, "Error unreachable evict kernel proc. Retrying.")
				time.Sleep(100 * time.Millisecond)
			}
			db.DPrintf(db.KERNEL, "Evicted %v", pid)
		}
	}
	// A purely sigmaclntd kernel won't have a procdir
	if !k.IsPurelySigmaclntdKernel() {
		if err := k.RmDir(k.ProcEnv().ProcDir); err != nil {
			db.DPrintf(db.KERNEL, "Failed to clean up %v err %v", k.ProcEnv().ProcDir, err)
		}
	}
	db.DPrintf(db.KERNEL, "Shutdown nameds %d\n", len(k.svcs.svcs[sp.KNAMED]))
	for _, ss := range k.svcs.svcs[sp.KNAMED] {
		// kill it so that test terminates;  XXX should this be  d.Terminate()?
		db.DPrintf(db.KERNEL, "Kill %v %v\n", sp.KNAMED, ss)
		ss.Kill()
		// d.Wait()
	}
	db.DPrintf(db.KERNEL, "Shutdown nameds done %d\n", len(k.svcs.svcs[sp.KNAMED]))
}

func newKNamedProc(realmId sp.Trealm, init bool) (*proc.Proc, error) {
	i := "start"
	if init {
		i = "init"
	}
	args := []string{realmId.String(), i}
	p := proc.NewPrivProcPid(sp.GenPid("knamed"), "knamed", args, true)
	return p, nil
}

// Run knamed (but not as a proc)
func runKNamed(pe *proc.ProcEnv, p *proc.Proc, realmId sp.Trealm, init bool) (*exec.Cmd, error) {
	r1, w1, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	r2, w2, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	p.InheritParentProcEnv(pe)
	cmd, err := kproc.RunKernelProc(pe.GetInnerContainerIP(), p, []*os.File{w1, r2, w2})
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
