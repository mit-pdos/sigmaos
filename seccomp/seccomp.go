package seccomp

import (
	"syscall"

	"github.com/seccomp/libseccomp-golang"
)

// Load seccomp filter whitelisting the system calls in whitelist.go
// and failing all others. (Note: NewFilter enables TSync so all
// goroutines will hopefully receive the filter).
func LoadFilter() error {
	filter, err := seccomp.NewFilter(seccomp.ActErrno.SetReturnCode(int16(syscall.EPERM)))
	if err != nil {
		return err
	}
	for _, name := range whitelist {
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
