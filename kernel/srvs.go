package kernel

import (
	"fmt"
	"path"
	"sync"

	db "sigmaos/debug"
	"sigmaos/port"
	"sigmaos/procclnt"
	sp "sigmaos/sigmap"
)

type Services struct {
	sync.Mutex
	svcs   map[string][]*Subsystem
	svcMap map[sp.Tpid]*Subsystem
}

func mkServices() *Services {
	ss := &Services{}
	ss.svcs = make(map[string][]*Subsystem)
	ss.svcMap = make(map[sp.Tpid]*Subsystem)
	return ss
}

func (ss *Services) addSvc(s string, sub *Subsystem) {
	ss.Lock()
	defer ss.Unlock()
	ss.svcs[s] = append(ss.svcs[s], sub)
	ss.svcMap[sub.p.GetPid()] = sub
}

func (k *Kernel) BootSub(s string, args []string, p *Param, full bool) (sp.Tpid, error) {
	var err error
	var ss *Subsystem
	switch s {
	case sp.NAMEDREL:
		ss, err = k.bootNamed()
	case sp.S3REL:
		ss, err = k.bootS3d()
	case sp.UXREL:
		ss, err = k.bootUxd()
	case sp.DBREL:
		ss, err = k.bootDbd(p.Dbip)
	case sp.MONGOREL:
		ss, err = k.bootMongod(p.Mongoip)
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
	return ss.p.GetPid(), err
}

func (k *Kernel) SetCPUShares(pid sp.Tpid, shares int64) error {
	return k.svcs.svcMap[pid].SetCPUShares(shares)
}

func (k *Kernel) GetCPUUtil(pid sp.Tpid) (float64, error) {
	return k.svcs.svcMap[pid].GetCPUUtil()
}

func (k *Kernel) AllocPort(pid sp.Tpid, port port.Tport) (*port.PortBinding, error) {
	return k.svcs.svcMap[pid].AllocPort(port)
}

func (k *Kernel) KillOne(srv string) error {
	k.Lock()
	defer k.Unlock()

	db.DPrintf(db.KERNEL, "KillOne %v\n", srv)

	var ss *Subsystem
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

func (k *Kernel) bootKNamed(init bool) error {
	p, err := makeKNamedProc(sp.ROOTREALM, init)
	if err != nil {
		return err
	}
	cmd, err := runKNamed(k.scfg, p, sp.ROOTREALM, init)
	if err != nil {
		return err
	}
	ss := makeSubsystemCmd(nil, k, p, procclnt.HLINUX, cmd)
	k.svcs.Lock()
	defer k.svcs.Unlock()
	k.svcs.svcs[sp.KNAMED] = append(k.svcs.svcs[sp.KNAMED], ss)
	return err
}

func (k *Kernel) bootRealmd() (*Subsystem, error) {
	return k.bootSubsystem("realmd", []string{}, procclnt.HSCHEDD)
}

func (k *Kernel) bootUxd() (*Subsystem, error) {
	// XXX ignore realm for now
	return k.bootSubsystem("fsuxd", []string{sp.SIGMAHOME}, procclnt.HSCHEDD)
}

func (k *Kernel) bootS3d() (*Subsystem, error) {
	// XXX Mount realm buckets dynamically.
	return k.bootSubsystem("fss3d", []string{"arielck", "kaashoek", "fkaashoek", "yizhengh"}, procclnt.HSCHEDD)
}

func (k *Kernel) bootDbd(hostip string) (*Subsystem, error) {
	return k.bootSubsystem("dbd", []string{hostip}, procclnt.HSCHEDD)
}

func (k *Kernel) bootMongod(hostip string) (*Subsystem, error) {
	return k.bootSubsystem("mongod", []string{hostip}, procclnt.HSCHEDD)
}

func (k *Kernel) bootSchedd() (*Subsystem, error) {
	return k.bootSubsystem("schedd", []string{k.Param.KernelId}, procclnt.HLINUX)
}

func (k *Kernel) bootNamed() (*Subsystem, error) {
	return k.bootSubsystem("named", []string{sp.ROOTREALM.String(), "0"}, procclnt.HSCHEDD)
}

// Start uprocd in a sigmauser container and post the mount for
// uprocd.  Uprocd cannot post because it doesn't know what the host
// IP address and port number are for it.
func (k *Kernel) bootUprocd(args []string) (*Subsystem, error) {
	s, err := k.bootSubsystem("uprocd", args, procclnt.HDOCKER)
	if err != nil {
		return nil, err
	}
	if s.k.Param.Overlays {
		realm := args[0]
		ptype := args[1]

		pn := path.Join(sp.SCHEDD, args[2], sp.UPROCDREL, realm, ptype)

		// container's first port is for uprocd
		pm, err := s.container.AllocFirst()
		if err != nil {
			return nil, err
		}

		// Use 127.0.0.1, because only the local schedd should be talking
		// to uprocd.
		mnt := sp.MkMountServer("127.0.0.1:" + pm.HostPort.String())
		db.DPrintf(db.BOOT, "Advertise %s at %v\n", pn, mnt)
		if err := k.MkMountSymlink(pn, mnt, sp.NoLeaseId); err != nil {
			return nil, err
		}
		db.DPrintf(db.KERNEL, "bootUprocd: started %v %s at %s, %v\n", realm, ptype, pn, pm)
	}

	return s, nil
}
