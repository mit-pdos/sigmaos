package kernel

import (
	"fmt"
	"path"

	"sigmaos/auth"
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

func (k *Kernel) BootSub(s string, args []string, p *Param, full bool) (sp.Tpid, error) {
	k.Lock()
	defer k.Unlock()

	if k.shuttingDown {
		return sp.Tpid(""), fmt.Errorf("Shutting down")
	}

	var err error
	var ss Subsystem
	switch s {
	case sp.NAMEDREL:
		ss, err = k.bootNamed()
	case sp.SIGMACLNTDREL:
		ss, err = k.bootSigmaclntd()
	case sp.S3REL:
		ss, err = k.bootS3d()
	case sp.UXREL:
		ss, err = k.bootUxd()
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
		return sp.Tpid(""), err
	}
	k.svcs.addSvc(s, ss)
	return ss.GetProc().GetPid(), err
}

func (k *Kernel) SetCPUShares(pid sp.Tpid, shares int64) error {
	return k.svcs.svcMap[pid].SetCPUShares(shares)
}

func (k *Kernel) AssignUprocdToRealm(pid sp.Tpid, realm sp.Trealm, ptype proc.Ttype) error {
	return k.svcs.svcMap[pid].AssignToRealm(realm, ptype)
}

func (k *Kernel) GetCPUUtil(pid sp.Tpid) (float64, error) {
	return k.svcs.svcMap[pid].GetCPUUtil()
}

func (k *Kernel) AllocPort(pid sp.Tpid, port sp.Tport) (*port.PortBinding, error) {
	return k.svcs.svcMap[pid].AllocPort(port)
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

func (k *Kernel) bootKNamed(pcfg *proc.ProcEnv, init bool) error {
	p, err := newKNamedProc(sp.ROOTREALM, init)
	if err != nil {
		return err
	}
	pc := auth.NewProcClaims(p.GetProcEnv())
	pc.AllowedPaths = []string{"*"}
	token, err := k.as.NewToken(pc)
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewToken: %v", err)
		return err
	}
	p.SetToken(token)
	cmd, err := runKNamed(pcfg, p, sp.ROOTREALM, init)
	if err != nil {
		return err
	}
	ss := newSubsystemCmd(nil, k, p, proc.HLINUX, cmd)
	k.svcs.svcs[sp.KNAMED] = append(k.svcs.svcs[sp.KNAMED], ss)
	return err
}

func (k *Kernel) bootRealmd() (Subsystem, error) {
	return k.bootSubsystem("realmd", []string{}, proc.HSCHEDD)
}

func (k *Kernel) bootUxd() (Subsystem, error) {
	// XXX ignore realm for now
	return k.bootSubsystem("fsuxd", []string{sp.SIGMAHOME}, proc.HSCHEDD)
}

func (k *Kernel) bootS3d() (Subsystem, error) {
	return k.bootSubsystem("fss3d", []string{}, proc.HSCHEDD)
}

func (k *Kernel) bootDbd(hostip string) (Subsystem, error) {
	return k.bootSubsystem("dbd", []string{hostip}, proc.HSCHEDD)
}

func (k *Kernel) bootMongod(hostip string) (Subsystem, error) {
	return k.bootSubsystemWithMcpu("mongod", []string{hostip}, proc.HSCHEDD, 1000)
}

func (k *Kernel) bootLCSched() (Subsystem, error) {
	return k.bootSubsystem("lcsched", []string{}, proc.HLINUX)
}

func (k *Kernel) bootProcq() (Subsystem, error) {
	return k.bootSubsystem("procq", []string{}, proc.HLINUX)
}

func (k *Kernel) bootSchedd() (Subsystem, error) {
	return k.bootSubsystem("schedd", []string{k.Param.KernelId, k.Param.ReserveMcpu}, proc.HLINUX)
}

func (k *Kernel) bootNamed() (Subsystem, error) {
	return k.bootSubsystem("named", []string{sp.ROOTREALM.String(), "0"}, proc.HSCHEDD)
}

func (k *Kernel) bootSigmaclntd() (Subsystem, error) {
	return sigmaclntsrv.ExecSigmaClntSrv()
}

// Start uprocd in a sigmauser container and post the mount for
// uprocd.  Uprocd cannot post because it doesn't know what the host
// IP address and port number are for it.
func (k *Kernel) bootUprocd(args []string) (Subsystem, error) {
	s, err := k.bootSubsystem("uprocd", args, proc.HDOCKER)
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
		mnt := sp.NewMountServer(addr)
		db.DPrintf(db.BOOT, "Advertise %s at %v\n", pn, mnt)
		if err := k.MkMountFile(pn, mnt, sp.NoLeaseId); err != nil {
			return nil, err
		}
		db.DPrintf(db.KERNEL, "bootUprocd: started %v at %s", pn, pm)
	}

	return s, nil
}
