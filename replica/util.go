package replica

import (
	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/netsrv"
	"ulambda/protclnt"
)

type SrvReplica interface {
	GetAddr() string
	GetPort() string
	GetConfigPath() string
	GetUnionDirPath() string
	GetSymlinkPath() string
	GetServiceName() string
}

func getConfig(r SrvReplica) *netsrv.NetServerReplConfig {
	fsl := fslib.MakeFsLib(r.GetServiceName() + "-replica:" + r.GetPort())
	clnt := protclnt.MakeClnt()
	config, err := netsrv.ReadReplConfig(r.GetConfigPath(), r.GetAddr(), fsl, clnt)
	// Reread until successful
	for err != nil {
		db.DLPrintf("RSRV", "Couldn't read repl config: %v\n", err)
		config, err = netsrv.ReadReplConfig(r.GetConfigPath(), r.GetAddr(), fsl, clnt)
	}
	config.UnionDirPath = r.GetUnionDirPath()
	config.SymlinkPath = r.GetSymlinkPath()
	return config
}
