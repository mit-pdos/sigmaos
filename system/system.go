package system

import (
	"os"
	"os/exec"
	"syscall"

	// db "sigmaos/debug"
	"sigmaos/realm"
	"sigmaos/realmv1"
)

type System struct {
	*realm.RealmClnt
	root  *realmv1.Realm // the sigma root realm
	realm *realmv1.Realm // XXX should be slice of realms
	proxy *exec.Cmd
}

func Boot(realmid string) (*System, error) {
	sys := &System{}
	r, err := realmv1.BootRealm(realmv1.ROOTREALM, "bootkernelclnt/bootsys.yml")
	if err != nil {
		return nil, err
	}
	sys.root = r
	sys.proxy = startProxy(sys.root.GetIP(), sys.root.NamedAddr())
	if err := sys.proxy.Start(); err != nil {
		return nil, err
	}
	//r, err = realmv1.BootRealm(realmid, "bootkernelclnt/bootall.yml")
	//if err != nil {
	//return nil, err
	//}
	sys.realm = r
	//cfg := e.CreateRealm(e.rid)
	// return cfg, nil
	return sys, nil
}

func startProxy(IP string, nds []string) *exec.Cmd {
	cmd := exec.Command("proxyd", append([]string{IP}, nds...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}
