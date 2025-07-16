package srv

import (
	"sigmaos/proc"
)

func RunCacheSrvBackup(args []string, nshard int) error {
	pn := ""
	if len(args) > 2 {
		pn = args[2]
	}

	pe := proc.GetProcEnv()
	s, err := NewCacheSrv(pe, args[1], pn, nshard)
	if err != nil {
		return err
	}

	s.ssrv.RunServer()
	s.exitCacheSrv()
	return nil
}
