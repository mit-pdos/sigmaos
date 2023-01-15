package kernel

import (
	"fmt"
	"path"
	"strconv"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
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

func (k *Kernel) BootSub(s string, p *Param, full bool) error {
	var err error
	var ss *Subsystem
	switch s {
	case sp.PROCDREL:
		ss, err = k.bootProcd(full)
	case sp.S3REL:
		ss, err = k.BootFss3d()
	case sp.UXREL:
		ss, err = k.BootFsUxd()
	case sp.DBREL:
		ss, err = k.BootDbd(p.Hostip)
	case sp.SIGMAMGRREL:
		ss, err = k.BootSigmaMgr()
	case sp.SCHEDDREL:
		ss, err = k.BootSchedd()
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

// replicaId is used to index into the fslib.Named() slice and select
// an address for this named.
func bootNamed(k *Kernel, uname string, replicaId int, realmId string) error {
	// replicaId needs to be 1-indexed for replication library.
	cmd, err := RunNamed(k.namedAddr[replicaId], len(k.namedAddr) > 1, replicaId+1, k.namedAddr, realmId)
	if err != nil {
		return err
	}
	ss := makeSubsystemCmd(nil, nil, realmId, "", false, cmd)
	k.svcs.Lock()
	defer k.svcs.Unlock()
	k.svcs.svcs[sp.NAMEDREL] = append(k.svcs.svcs[sp.NAMEDREL], ss)

	time.Sleep(SLEEP_MS * time.Millisecond)
	return err
}

func (k *Kernel) BootProcd() (*Subsystem, error) {
	return k.bootProcd(false)
}

// Boot a procd. If spawningSys is true, procd will wait for all kernel procs
// to be spawned before claiming any procs.
func (k *Kernel) bootProcd(spawningSys bool) (*Subsystem, error) {
	ss, err := k.bootSubsystem("procd", []string{k.Param.Realm, strconv.FormatBool(spawningSys)}, k.Param.Realm, "", false)
	if err != nil {
		return nil, err
	}
	if k.procdIp == "" {
		k.procdIp = ss.GetIp(k.FsLib)
	}
	return ss, nil
}

func (k *Kernel) BootFsUxd() (*Subsystem, error) {
	return k.bootSubsystem("fsuxd", []string{path.Join(sp.SIGMAHOME, k.Param.Realm)}, k.Param.Realm, k.procdIp, true)
}

func (k *Kernel) BootFss3d() (*Subsystem, error) {
	return k.bootSubsystem("fss3d", []string{k.Param.Realm}, k.Param.Realm, k.procdIp, true)
}

func (k *Kernel) BootDbd(hostip string) (*Subsystem, error) {
	return k.bootSubsystem("dbd", []string{hostip + ":3306"}, k.Param.Realm, k.procdIp, true)
}

func (k *Kernel) BootSigmaMgr() (*Subsystem, error) {
	return k.bootSubsystem("sigmamgr", []string{}, k.Param.Realm, k.procdIp, false)
}

func (k *Kernel) BootSchedd() (*Subsystem, error) {
	return k.bootSubsystem("schedd", []string{}, k.Param.Realm, k.procdIp, false)
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
