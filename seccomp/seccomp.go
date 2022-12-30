package seccomp

import (
	"syscall"

	"github.com/seccomp/libseccomp-golang"

	"sigmaos/yaml"
)

type WhiteList struct {
	Allowed []string `yalm:"allowed"`
}

func ReadWhiteList(pn string) (*WhiteList, error) {
	wl := &WhiteList{}
	if err := yaml.ReadYaml(pn, wl); err != nil {
		return nil, err
	}
	return wl, nil
}

// Load seccomp filter whitelisting the system calls in whitelist.go
// and failing all others. (Note: NewFilter enables TSync so all
// goroutines will hopefully receive the filter).
func LoadFilter(wl *WhiteList) error {
	filter, err := seccomp.NewFilter(seccomp.ActErrno.SetReturnCode(int16(syscall.EPERM)))
	if err != nil {
		return err
	}
	for _, name := range wl.Allowed {
		syscallID, err := seccomp.GetSyscallFromName(name)
		if err != nil {
			return err
		}
		err = filter.AddRule(syscallID, seccomp.ActAllow)
		if err != nil {
			return err
		}
	}
	return filter.Load()
}
