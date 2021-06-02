package replica

import (
	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/npclnt"
	"ulambda/npsrv"
)

type SrvReplica interface {
	GetAddr() string
	GetPort() string
	GetConfigPath() string
	GetUnionDirPath() string
	GetServiceName() string
}

func getConfig(r SrvReplica) *npsrv.NpServerReplConfig {
	fsl := fslib.MakeFsLib(r.GetServiceName() + "-replica:" + r.GetPort())
	clnt := npclnt.MakeNpClnt()
	config, err := npsrv.ReadReplConfig(r.GetConfigPath(), r.GetAddr(), fsl, clnt)
	// Reread until successful
	for err != nil {
		db.DLPrintf("RSRV", "Couldn't read repl config: %v\n", err)
		config, err = npsrv.ReadReplConfig(r.GetConfigPath(), r.GetAddr(), fsl, clnt)
	}
	config.UnionDirPath = r.GetUnionDirPath()
	return config
}
