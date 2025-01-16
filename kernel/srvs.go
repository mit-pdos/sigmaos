package kernel

import (
	"fmt"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/proc"
	spproxysrv "sigmaos/proxy/sigmap/srv"
	sp "sigmaos/sigmap"
)

type Services struct {
	svcs   map[string][]Subsystem
	svcMap map[sp.Tpid]Subsystem
}

func newServices() *Services {
	ss := &Services{}
	ss.svcs = make(map[string][]Subsystem)
	ss.svcMap = make(map[sp.Tpid]Subsystem)
	return ss
}

func (ss *Services) addSvc(s string, sub Subsystem) {
	ss.svcs[s] = append(ss.svcs[s], sub)
	ss.svcMap[sub.GetProc().GetPid()] = sub
}

func (k *Kernel) BootSub(s string, args, env []string, p *Param, realm sp.Trealm) (sp.Tpid, error) {
	db.DPrintf(db.KERNEL, "Boot sub %v realm %v", s, realm)
	defer db.DPrintf(db.KERNEL, "Boot sub %v done realm %v", s, realm)

	k.Lock()
	defer k.Unlock()

	if k.shuttingDown {
		return sp.NO_PID, fmt.Errorf("Shutting down")
	}

	var err error
	var ss Subsystem
	switch s {
	case sp.NAMEDREL:
		ss, err = k.bootNamed(env)
	case sp.SPPROXYDREL:
		ss, err = k.bootSPProxyd()
	case sp.S3REL:
		ss, err = k.bootS3d(realm)
	case sp.CHUNKDREL:
		ss, err = k.bootChunkd(realm)
	case sp.UXREL:
		ss, err = k.bootUxd(realm, env)
	case sp.DBREL:
		ss, err = k.bootDbd(p.Dbip)
	case sp.MONGOREL:
		ss, err = k.bootMongod(p.Mongoip)
	case sp.LCSCHEDREL:
		ss, err = k.bootLCSched()
	case sp.BESCHEDREL:
		ss, err = k.bootBESched()
	case sp.MSCHEDREL:
		ss, err = k.bootMSched(env)
	case sp.REALMDREL:
		ss, err = k.bootRealmd()
	case sp.PROCDREL:
		ss, err = k.bootProcd(args)
	default:
		err = fmt.Errorf("bootSub: unknown srv %s\n", s)
	}
	if err != nil {
		return sp.NO_PID, err
	}
	k.svcs.addSvc(s, ss)
	return ss.GetProc().GetPid(), err
}

func (k *Kernel) SetCPUShares(pid sp.Tpid, shares int64) error {
	return k.svcs.svcMap[pid].SetCPUShares(shares)
}

func (k *Kernel) GetCPUUtil(pid sp.Tpid) (float64, error) {
	return k.svcs.svcMap[pid].GetCPUUtil()
}

func (k *Kernel) EvictKernelProc(pid sp.Tpid) error {
	k.Lock()
	defer k.Unlock()

	db.DPrintf(db.KERNEL, "Evict kernel proc %v", pid)
	// Evict the kernel proc
	if err := k.svcs.svcMap[pid].Evict(); err != nil {
		return err
	}
	db.DPrintf(db.KERNEL, "Wait for evicted kernel proc %v", pid)
	// Wait for it to exit
	return k.svcs.svcMap[pid].Wait()
}

func (k *Kernel) KillOne(srv string) error {
	k.Lock()
	defer k.Unlock()

	db.DPrintf(db.KERNEL, "KillOne %v\n", srv)

	var ss Subsystem
	if _, ok := k.svcs.svcs[srv]; !ok {
		return fmt.Errorf("Unknown kernel service %v", srv)
	}
	if len(k.svcs.svcs[srv]) > 0 {
		ss = k.svcs.svcs[srv][0]
		k.svcs.svcs[srv] = k.svcs.svcs[srv][1:]
	} else {
		return fmt.Errorf("Tried to kill %s, nothing to kill", srv)
	}
	if err := ss.Kill(); err == nil {
		if err := ss.Wait(); err != nil {
			db.DPrintf(db.ALWAYS, "KillOne: Kill %v err %v\n", ss, err)
		}
		return nil
	} else {
		return fmt.Errorf("Kill %v err %v", srv, err)
	}
}

func (k *Kernel) bootKNamed(pe *proc.ProcEnv, init bool) error {
	p, err := newKNamedProc(sp.ROOTREALM, init)
	if err != nil {
		return err
	}
	p.SetRealm(sp.ROOTREALM)
	p.SetKernelID(k.Param.KernelID, false)
	cmd, err := runKNamed(pe, p, sp.ROOTREALM, init)
	if err != nil {
		return err
	}
	ss := newSubsystemCmd(nil, k, p, proc.HLINUX, cmd)
	k.svcs.svcs[sp.KNAMED] = append(k.svcs.svcs[sp.KNAMED], ss)
	return err
}

func (k *Kernel) bootRealmd() (Subsystem, error) {
	return k.bootSubsystem("realmd", []string{strconv.FormatBool(k.Param.DialProxy)}, []string{}, sp.ROOTREALM, proc.HLINUX, 0)
}

func (k *Kernel) bootUxd(realm sp.Trealm, env []string) (Subsystem, error) {
	return k.bootSubsystem("fsuxd", []string{sp.SIGMAHOME}, env, realm, proc.HMSCHED, 0)
}

func (k *Kernel) bootS3d(realm sp.Trealm) (Subsystem, error) {
	return k.bootSubsystem("fss3d", []string{}, []string{}, realm, proc.HMSCHED, 0)
}

func (k *Kernel) bootChunkd(realm sp.Trealm) (Subsystem, error) {
	return k.bootSubsystem("chunkd", []string{k.Param.KernelID}, []string{}, realm, proc.HMSCHED, 0)
}

func (k *Kernel) bootDbd(hostip string) (Subsystem, error) {
	return k.bootSubsystem("dbd", []string{hostip}, []string{}, sp.ROOTREALM, proc.HMSCHED, 0)
}

func (k *Kernel) bootMongod(hostip string) (Subsystem, error) {
	return k.bootSubsystem("mongod", []string{hostip}, []string{}, sp.ROOTREALM, proc.HMSCHED, 1000)
}

func (k *Kernel) bootLCSched() (Subsystem, error) {
	return k.bootSubsystem("lcsched", []string{}, []string{}, sp.ROOTREALM, proc.HLINUX, 0)
}

func (k *Kernel) bootBESched() (Subsystem, error) {
	return k.bootSubsystem("besched", []string{}, []string{}, sp.ROOTREALM, proc.HLINUX, 0)
}
func (k *Kernel) bootMSched(env []string) (Subsystem, error) {
	return k.bootSubsystem("msched", []string{k.Param.KernelID, k.Param.ReserveMcpu}, env, sp.ROOTREALM, proc.HLINUX, 0)
}

func (k *Kernel) bootNamed(env []string) (Subsystem, error) {
	return k.bootSubsystem("named", []string{sp.ROOTREALM.String()}, env, sp.ROOTREALM, proc.HMSCHED, 0)
}

func (k *Kernel) bootSPProxyd() (Subsystem, error) {
	pid := sp.GenPid("spproxy")
	p := proc.NewPrivProcPid(pid, "spproxy", nil, true)
	p.GetProcEnv().SetSecrets(k.ProcEnv().GetSecrets())
	p.SetHow(proc.HLINUX)
	p.InheritParentProcEnv(k.ProcEnv())
	return spproxysrv.ExecSPProxySrv(p, k.ProcEnv().GetInnerContainerIP(), k.ProcEnv().GetOuterContainerIP(), sp.Tpid("NO_PID"))
}

// Start procd in a sigmauser container and post the mount for
// procd.  Procd cannot post because it doesn't know what the host
// IP address and port number are for it.
func (k *Kernel) bootProcd(args []string) (Subsystem, error) {
	spproxydPID := sp.GenPid("spproxyd")
	// Append args
	args = append(args, strconv.FormatBool(k.Param.DialProxy), spproxydPID.String())
	db.DPrintf(db.ALWAYS, "Procd args %v", args)
	s, err := k.bootSubsystem("procd", args, []string{}, sp.ROOTREALM, proc.HDOCKER, 0)
	if err != nil {
		return nil, err
	}
	return s, nil
}
