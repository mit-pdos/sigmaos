package kernel

import (
	"fmt"
	"path"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/port"
	"sigmaos/proc"
	"sigmaos/sigmaclntsrv"
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

func (k *Kernel) BootSub(s string, args []string, p *Param, realm sp.Trealm) (sp.Tpid, error) {
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
		ss, err = k.bootNamed()
	case sp.SIGMACLNTDREL:
		ss, err = k.bootSigmaclntd()
	case sp.S3REL:
		ss, err = k.bootS3d(realm)
	case sp.CHUNKDREL:
		ss, err = k.bootChunkd(realm)
	case sp.UXREL:
		ss, err = k.bootUxd(realm)
	case sp.DBREL:
		ss, err = k.bootDbd(p.Dbip)
	case sp.MONGOREL:
		ss, err = k.bootMongod(p.Mongoip)
	case sp.LCSCHEDREL:
		ss, err = k.bootLCSched()
	case sp.PROCQREL:
		ss, err = k.bootProcq()
	case sp.SCHEDDREL:
		ss, err = k.bootSchedd()
	case sp.REALMDREL:
		ss, err = k.bootRealmd()
	case sp.UPROCDREL:
		ss, err = k.bootUprocd(args)
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

func (k *Kernel) AssignUprocdToRealm(pid sp.Tpid, realm sp.Trealm, ptype proc.Ttype) error {
	err := k.svcs.svcMap[pid].AssignToRealm(realm, ptype)
	if err != nil {
		db.DPrintf(db.ERROR, "Error assign uprocd to realm: %v", err)
		return err
	}
	return nil
}

func (k *Kernel) GetCPUUtil(pid sp.Tpid) (float64, error) {
	return k.svcs.svcMap[pid].GetCPUUtil()
}

func (k *Kernel) AllocPort(pid sp.Tpid, port sp.Tport) (*port.PortBinding, error) {
	return k.svcs.svcMap[pid].AllocPort(port)
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
	p.GetProcEnv().SetRealm(sp.ROOTREALM, k.Param.Overlays)
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
	return k.bootSubsystem("realmd", []string{strconv.FormatBool(k.Param.NetProxy)}, sp.ROOTREALM, proc.HSCHEDD, 0)
}

func (k *Kernel) bootUxd(realm sp.Trealm) (Subsystem, error) {
	return k.bootSubsystem("fsuxd", []string{sp.SIGMAHOME}, realm, proc.HSCHEDD, 0)
}

func (k *Kernel) bootS3d(realm sp.Trealm) (Subsystem, error) {
	return k.bootSubsystem("fss3d", []string{}, realm, proc.HSCHEDD, 0)
}

func (k *Kernel) bootChunkd(realm sp.Trealm) (Subsystem, error) {
	return k.bootSubsystem("chunkd", []string{k.Param.KernelID}, realm, proc.HSCHEDD, 0)
}

func (k *Kernel) bootDbd(hostip string) (Subsystem, error) {
	return k.bootSubsystem("dbd", []string{hostip}, sp.ROOTREALM, proc.HSCHEDD, 0)
}

func (k *Kernel) bootMongod(hostip string) (Subsystem, error) {
	return k.bootSubsystem("mongod", []string{hostip}, sp.ROOTREALM, proc.HSCHEDD, 1000)
}

func (k *Kernel) bootLCSched() (Subsystem, error) {
	return k.bootSubsystem("lcsched", []string{}, sp.ROOTREALM, proc.HLINUX, 0)
}

func (k *Kernel) bootProcq() (Subsystem, error) {
	return k.bootSubsystem("procq", []string{}, sp.ROOTREALM, proc.HLINUX, 0)
}
func (k *Kernel) bootSchedd() (Subsystem, error) {
	return k.bootSubsystem("schedd", []string{k.Param.KernelID, k.Param.ReserveMcpu}, sp.ROOTREALM, proc.HLINUX, 0)
}

func (k *Kernel) bootNamed() (Subsystem, error) {
	return k.bootSubsystem("named", []string{sp.ROOTREALM.String(), "0"}, sp.ROOTREALM, proc.HSCHEDD, 0)
}

func (k *Kernel) bootSigmaclntd() (Subsystem, error) {
	pid := sp.GenPid("sigmaclntd")
	p := proc.NewPrivProcPid(pid, "sigmaclntd", nil, true)
	p.GetProcEnv().SetSecrets(k.ProcEnv().GetSecrets())
	p.SetHow(proc.HLINUX)
	p.InheritParentProcEnv(k.ProcEnv())
	return sigmaclntsrv.ExecSigmaClntSrv(p, k.ProcEnv().GetInnerContainerIP(), k.ProcEnv().GetOuterContainerIP(), sp.Tpid("NO_PID"))
}

// Start uprocd in a sigmauser container and post the mount for
// uprocd.  Uprocd cannot post because it doesn't know what the host
// IP address and port number are for it.
func (k *Kernel) bootUprocd(args []string) (Subsystem, error) {
	// Append netproxy bool to args
	args = append(args, strconv.FormatBool(k.Param.NetProxy))
	sigmaclntdPID := sp.GenPid("sigmaclntd")
	sigmaclntdArgs := append([]string{sigmaclntdPID.String()})
	db.DPrintf(db.ALWAYS, "Uprocd args %v", args)
	s, err := k.bootSubsystem("uprocd", append(args, sigmaclntdArgs...), sp.ROOTREALM, proc.HDOCKER, 0)
	if err != nil {
		return nil, err
	}
	if k.Param.Overlays {
		pn := path.Join(sp.SCHEDD, args[0], sp.UPROCDREL, s.GetProc().GetPid().String())

		// container's first port is for uprocd
		pm, err := s.GetContainer().AllocFirst()
		if err != nil {
			return nil, err
		}

		// Use 127.0.0.1, because only the local schedd should be talking
		// to uprocd.
		addr := sp.NewTaddr(sp.LOCALHOST, sp.INNER_CONTAINER_IP, pm.HostPort)
		ep := sp.NewEndpoint(sp.INTERNAL_EP, []*sp.Taddr{addr}, sp.ROOTREALM)
		db.DPrintf(db.BOOT, "Advertise %s at %v\n", pn, ep)
		if err := k.MkEndpointFile(pn, ep, sp.NoLeaseId); err != nil {
			return nil, err
		}
		// Alloc a port for WWW srvs running on this uprocd
		pm2, err := s.GetContainer().AllocPortOne(PUBLIC_PORT)
		if err != nil {
			return nil, err
		}
		portFN := path.Join(pn, sp.PUBLIC_PORT)
		if err := k.PutFileJson(portFN, 0777, pm2); err != nil {
			db.DPrintf(db.ERROR, "Error put public port file: %v", err)
			return nil, err
		}
		db.DPrintf(db.KERNEL, "bootUprocd: started %v at %s pfn %v", pn, pm, portFN)
	}
	return s, nil
}
