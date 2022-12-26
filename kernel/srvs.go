package kernel

import (
	"fmt"
	"os"
	"path"
	"strconv"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

func (k *Kernel) BootSub(s string) error {
	var err error
	switch s {
	case sp.PROCDREL:
		err = k.BootProcd()
	case sp.S3REL:
		err = k.BootFss3d()
	case sp.UXREL:
		err = k.BootFsUxd()
	case sp.DBREL:
		err = k.BootDbd()
	default:
		err = fmt.Errorf("bootSub: unknown srv %s\n", s)
	}
	return err
}

func (k *Kernel) BootProcd() error {
	return k.bootProcd(false)
}

// Boot a procd. If spawningSys is true, procd will wait for all kernel procs
// to be spawned before claiming any procs.
func (k *Kernel) bootProcd(spawningSys bool) error {
	err := k.bootSubsystem("kernel/procd", []string{path.Join(k.realmId, "bin"), k.cores.Marshal(), strconv.FormatBool(spawningSys)}, "", false, &k.procd)
	if err != nil {
		return err
	}
	if k.procdIp == "" {
		k.procdIp = k.GetProcdIp()
	}
	return nil
}

func (k *Kernel) BootFsUxd() error {
	return k.bootSubsystem("kernel/fsuxd", []string{path.Join(sp.UXROOT, k.realmId)}, k.procdIp, true, &k.fsuxd)
}

func (k *Kernel) BootFss3d() error {
	return k.bootSubsystem("kernel/fss3d", []string{k.realmId}, k.procdIp, true, &k.fss3d)
}

func (k *Kernel) BootDbd() error {
	var dbdaddr string
	dbdaddr = os.Getenv("SIGMADBADDR")
	// XXX don't pass dbd addr as an envvar, it's messy.
	if dbdaddr == "" {
		dbdaddr = "127.0.0.1:3306"
	}
	return k.bootSubsystem("kernel/dbd", []string{dbdaddr}, k.procdIp, true, &k.dbd)
	return nil
}

func (k *Kernel) GetProcdIp() string {
	k.Lock()
	defer k.Unlock()

	if len(k.procd) != 1 {
		db.DFatalf("Error unexpexted num procds: %v", k.procd)
	}
	return GetSubsystemInfo(k.FsLib, sp.KPIDS, k.procd[0].p.Pid.String()).Ip
}
