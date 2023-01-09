package system

import (
	"os"
	"os/exec"

	// db "sigmaos/debug"
	"sigmaos/realm"
	"sigmaos/realmv1"
)

type System struct {
	*realm.RealmClnt
	realm *realmv1.Realm // the sigma root realm
	proxy *exec.Cmd
}

func Boot() (*System, error) {
	sys := &System{}
	r, err := realmv1.BootRealm(realmv1.ROOTREALM, "bootkernelclnt/bootsys.yml")
	if err != nil {
		return nil, err
	}
	sys.realm = r
	sys.proxy = startProxy(r.GetIP(), r.NamedAddr())
	if err := sys.proxy.Start(); err != nil {
		return nil, err
	}
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
