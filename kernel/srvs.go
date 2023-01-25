package kernel

import (
	"fmt"
	// "path"
	"strconv"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/procclnt"
	sp "sigmaos/sigmap"
)

type Services struct {
	sync.Mutex
	svcs        map[string][]*Subsystem
	crashedPids map[proc.Tpid]bool
}

func mkServices() *Services {
	ss := &Services{}
	ss.svcs = make(map[string][]*Subsystem)
	ss.crashedPids = make(map[proc.Tpid]bool)
	return ss
}

func (ss *Services) addSvc(s string, sub *Subsystem) {
	ss.Lock()
	defer ss.Unlock()
	ss.svcs[s] = append(ss.svcs[s], sub)
}

func (k *Kernel) BootSub(s string, args []string, p *Param, full bool) error {
	var err error
	var ss *Subsystem
	switch s {
	case sp.PROCDREL:
		ss, err = k.bootProcd(full)
	case sp.S3REL:
		ss, err = k.bootS3d()
	case sp.UXREL:
		ss, err = k.bootUxd()
	case sp.DBREL:
		ss, err = k.bootDbd("172.17.0.1")
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
		return err
	}
	k.svcs.addSvc(s, ss)
	return err
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
		k.svcs.crashedPids[ss.p.GetPid()] = true
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
	ss := makeSubsystemCmd(nil, nil, realmId, "", procclnt.HLINUX, cmd)
	k.svcs.Lock()
	defer k.svcs.Unlock()
	k.svcs.svcs[sp.NAMEDREL] = append(k.svcs.svcs[sp.NAMEDREL], ss)

	time.Sleep(SLEEP_MS * time.Millisecond)
	return err
}

// Boot a procd. If spawningSys is true, procd will wait for all kernel procs
// to be spawned before claiming any procs.
func (k *Kernel) bootProcd(spawningSys bool) (*Subsystem, error) {
	ss, err := k.bootSubsystem("procd", []string{k.Param.Realm.String(), strconv.FormatBool(spawningSys)}, k.Param.Realm, "", procclnt.HLINUX)
	if err != nil {
		return nil, err
	}
	if k.procdIp == "" {
		k.procdIp = ss.GetIp(k.FsLib)
	}
	return ss, nil
}

func (k *Kernel) bootRealmd() (*Subsystem, error) {
	return k.bootSubsystem("realmd", []string{}, k.Param.Realm, k.procdIp, procclnt.HPROCD)
}

func (k *Kernel) bootUxd() (*Subsystem, error) {
	// XXX ignore realm for now
	return k.bootSubsystem("fsuxd", []string{sp.SIGMAHOME}, k.Param.Realm, k.procdIp, procclnt.HPROCD)
}

func (k *Kernel) bootS3d() (*Subsystem, error) {
	return k.bootSubsystem("fss3d", []string{k.Param.Realm.String()}, k.Param.Realm, k.procdIp, procclnt.HPROCD)
}

func (k *Kernel) bootDbd(hostip string) (*Subsystem, error) {
	return k.bootSubsystem("dbd", []string{hostip + ":3306"}, k.Param.Realm, k.procdIp, procclnt.HPROCD)
}

func (k *Kernel) bootSchedd() (*Subsystem, error) {
	return k.bootSubsystem("schedd", []string{}, k.Param.Realm, k.procdIp, procclnt.HLINUX)
}

func (k *Kernel) bootUprocd(args []string) (*Subsystem, error) {
	return k.bootSubsystem("uprocd", args, k.Param.Realm, k.procdIp, procclnt.HDOCKER)
}

func (k *Kernel) GetProcdIp() string {
	k.svcs.Lock()
	defer k.svcs.Unlock()

	if len(k.svcs.svcs[sp.PROCDREL]) != 1 {
		db.DFatalf("Error unexpexted num procds: %v", k.svcs.svcs[sp.PROCDREL])
	}
	procd := k.svcs.svcs[sp.PROCDREL][0]
	return procd.GetIp(k.FsLib)
}
