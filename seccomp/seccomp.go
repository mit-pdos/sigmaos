package seccomp

import (
	"fmt"
	"syscall"

	"github.com/seccomp/libseccomp-golang"

	"sigmaos/yaml"
)

type WhiteList struct {
	Allowed     []string         `yaml:"allowed"`
	CondAllowed map[string]*Cond `yaml:"cond_allowed"`
}

type Cond struct {
	Index uint   `yaml:"index"`
	Op1   uint64 `yaml:"op1"`
	Op2   uint64 `yaml:"op2"`
	Op    string `yaml:"op"`
}

func (wl *WhiteList) String() string {
	return fmt.Sprintf("{ Allowed:%v CondAllowed:%v }", wl.Allowed, wl.CondAllowed)
}

func (cond *Cond) String() string {
	return fmt.Sprintf("{ Idx:%v Op1:%v Op2:%v Op:%v }", cond.Index, cond.Op1, cond.Op2, cond.Op)
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
		// Add a rule for the syscall.
		err = filter.AddRule(syscallID, seccomp.ActAllow)
		if err != nil {
			return err
		}
	}
	for name, c := range wl.CondAllowed {
		syscallID, err := seccomp.GetSyscallFromName(name)
		if err != nil {
			return err
		}
		op, err := parseOp(c.Op)
		if err != nil {
			return err
		}
		cond, err := seccomp.MakeCondition(c.Index, op, c.Op1, c.Op2)
		if err != nil {
			return err
		}
		// Add a conditional rule for the syscall.
		err = filter.AddRuleConditional(syscallID, seccomp.ActAllow, []seccomp.ScmpCondition{cond})
		if err != nil {
			return err
		}
	}
	return filter.Load()
}

func parseOp(op string) (seccomp.ScmpCompareOp, error) {
	switch op {
	case "SCMP_CMP_NE":
		return seccomp.CompareNotEqual, nil
	case "SCMP_CMP_MASKED_EQ":
		return seccomp.CompareMaskedEqual, nil
	default:
		return 0, fmt.Errorf("Unrecognized seccomp op %v")
	}
}
