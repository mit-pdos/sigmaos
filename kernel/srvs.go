package kernel

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
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

func (k *Kernel) BootSub(s string) error {
	var err error
	var ss *Subsystem
	switch s {
	case sp.PROCDREL:
		ss, err = k.BootProcd()
	case sp.S3REL:
		ss, err = k.BootFss3d()
	case sp.UXREL:
		ss, err = k.BootFsUxd()
	case sp.DBREL:
		ss, err = k.BootDbd()
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
		k.svcs.crashedPids[ss.p.Pid] = true
	} else {
		db.DFatalf("%v kill failed %v\n", srv, err)
	}
	return nil
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
	ss, err := k.bootSubsystem("kernel/procd", []string{path.Join(k.realmId, "bin"), k.cores.Marshal(), strconv.FormatBool(spawningSys)}, "", false)
	if err != nil {
		return nil, err
	}
	if k.procdIp == "" {
		k.procdIp = ss.GetIp(k.FsLib)
	}
	return ss, nil
}

func (k *Kernel) BootFsUxd() (*Subsystem, error) {
	return k.bootSubsystem("kernel/fsuxd", []string{path.Join(sp.SIGMAROOT, k.realmId)}, k.procdIp, true)
}

func (k *Kernel) BootFss3d() (*Subsystem, error) {
	return k.bootSubsystem("kernel/fss3d", []string{k.realmId}, k.procdIp, true)
}

func (k *Kernel) BootDbd() (*Subsystem, error) {
	var dbdaddr string
	dbdaddr = os.Getenv("SIGMADBADDR")
	// XXX don't pass dbd addr as an envvar, it's messy.
	if dbdaddr == "" {
		// dbdaddr = "127.0.0.1:3306"
		dbdaddr = "192.168.0.9:3306"
	}
	return k.bootSubsystem("kernel/dbd", []string{dbdaddr}, k.procdIp, true)
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
