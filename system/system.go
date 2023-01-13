package system

import (
	"os"
	"os/exec"
	"path"
	"syscall"

	// db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/realm"
	"sigmaos/realmv1"
	sp "sigmaos/sigmap"
)

type System struct {
	Root  *realmv1.Realm // the sigma root realm
	realm *realmv1.Realm // XXX should be slice of realms
	proxy *exec.Cmd
}

func Boot(realmid, ymldir string) (*System, error) {
	sys := &System{}
	r, err := realmv1.BootRealmOld(realmv1.ROOTREALM, path.Join(ymldir, "bootsys.yml"))
	if err != nil {
		return nil, err
	}
	sys.Root = r
	sys.proxy = startProxy(sys.Root.GetIP(), sys.Root.NamedAddr())
	if err := sys.proxy.Start(); err != nil {
		return nil, err
	}
	r, err = realmv1.BootRealmOld(realmid, path.Join(ymldir, "bootall.yml"))
	if err != nil {
		return nil, err
	}
	sys.realm = r
	pn := path.Join(realm.REALM_NAMEDS, realmid)
	nameds, err := fslib.SetNamedIP(r.GetIP())
	if err != nil {
		return nil, err
	}
	mnt := sp.MkMountService(nameds)
	if err := sys.Root.MkMountSymlink(pn, mnt); err != nil {
		return nil, err
	}

	//cfg := e.CreateRealm(e.rid)
	// return cfg, nil
	return sys, nil
}

func (sys *System) Shutdown() error {
	if err := sys.realm.Shutdown(); err != nil {
		return err
	}
	if err := sys.Root.Shutdown(); err != nil {
		return err
	}
	if err := sys.proxy.Process.Kill(); err != nil {
		return err
	}
	return nil
}

func startProxy(IP string, nds []string) *exec.Cmd {
	cmd := exec.Command("proxyd", append([]string{IP}, nds...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}
