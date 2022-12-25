package kernel

import (
	db "sigmaos/debug"
	"sigmaos/system"
)

type Boot struct {
	s *system.System
}

// The boot processes enters here
func BootUp(pn string) (*Boot, error) {
	db.DPrintf(db.KERNEL, "Boot %s\n", pn)
	param, err := system.ReadParam(pn)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.KERNEL, "Boot %s param %v\n", pn, param)
	s, err := system.MakeKernel(param)
	if err != nil {
		return nil, err
	}
	return &Boot{s}, nil
}

func (b *Boot) ShutDown() error {
	return b.s.ShutDown()
}
