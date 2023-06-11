package kernel

import (
	"fmt"
	"path"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/port"
	"sigmaos/proc"
	"sigmaos/procclnt"
	sp "sigmaos/sigmap"
)

type Services struct {
	sync.Mutex
	svcs   map[string][]*Subsystem
	svcMap map[proc.Tpid]*Subsystem
}

func mkServices() *Services {
	ss := &Services{}
	ss.svcs = make(map[string][]*Subsystem)
	ss.svcMap = make(map[proc.Tpid]*Subsystem)
	return ss
}

func (ss *Services) addSvc(s string, sub *Subsystem) {
	ss.Lock()
	defer ss.Unlock()
	ss.svcs[s] = append(ss.svcs[s], sub)
	ss.svcMap[sub.p.GetPid()] = sub
}

func (k *Kernel) BootSub(s string, args []string, p *Param, full bool) (proc.Tpid, error) {
	var err error
	var ss *Subsystem
	switch s {
	case sp.S3REL:
		ss, err = k.bootS3d()
	case sp.UXREL:
		ss, err = k.bootUxd()
	case sp.DBREL:
		ss, err = k.bootDbd(p.Dbip)
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
		return proc.Tpid(""), err
	}
	k.svcs.addSvc(s, ss)
	return ss.p.GetPid(), err
}

func (k *Kernel) SetCPUShares(pid proc.Tpid, shares int64) error {
	return k.svcs.svcMap[pid].SetCPUShares(shares)
}

func (k *Kernel) GetCPUUtil(pid proc.Tpid) (float64, error) {
	return k.svcs.svcMap[pid].GetCPUUtil()
}

func (k *Kernel) AllocPort(pid proc.Tpid, port port.Tport) (*port.PortBinding, error) {
	return k.svcs.svcMap[pid].AllocPort(port)
}

func (k *Kernel) KillOne(srv string) error {
	k.Lock()
	defer k.Unlock()

	var ss *Subsystem
	if len(k.svcs.svcs[srv]) > 0 {
		ss = k.svcs.svcs[srv][0]
		k.svcs.svcs[srv] = k.svcs.svcs[srv][1:]
	} else {
		db.DPrintf(db.ALWAYS, "Tried to kill %s, nothing to kill", srv)
	}
	err := ss.Kill()
	if err == nil {
		ss.Wait()
	} else {
		db.DFatalf("%v kill failed %v\n", srv, err)
	}
	return nil
}

// replicaId is used to index into the namedAddr slice and select
// an address for this named.
func bootNamed(k *Kernel, uname string, replicaId int, realmId sp.Trealm) error {
	// replicaId needs to be 1-indexed for replication library.
	cmd, err := RunNamed(k.namedAddr[replicaId], len(k.namedAddr) > 1, replicaId+1, k.namedAddr, realmId)
	if err != nil {
		return err
	}
	ss := makeSubsystemCmd(nil, k, nil, procclnt.HLINUX, cmd)
	k.svcs.Lock()
	defer k.svcs.Unlock()
	k.svcs.svcs[sp.NAMEDREL] = append(k.svcs.svcs[sp.NAMEDREL], ss)

	time.Sleep(SLEEP_MS * time.Millisecond)
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
	return k.bootSubsystem("dbd", []string{hostip + ":3306"}, procclnt.HSCHEDD)
}

func (k *Kernel) bootSchedd() (*Subsystem, error) {
	return k.bootSubsystem("schedd", []string{k.Param.KernelId}, procclnt.HLINUX)
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
		if err := k.MkMountSymlink(pn, mnt); err != nil {
			return nil, err
		}
		db.DPrintf(db.KERNEL, "bootUprocd: started %v %s at %s, %v\n", realm, ptype, pn, pm)
	}

	return s, nil
}
