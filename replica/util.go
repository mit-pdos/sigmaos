package replica

import (
	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/protclnt"
	"ulambda/replchain"
)

func GetChainReplConfig(name, port, configPath, addr, unionDirPath, symlinkPath string) *replchain.ChainReplConfig {
	fsl := fslib.MakeFsLib(name + "-replica:" + port)
	clnt := protclnt.MakeClnt()
	config, err := replchain.ReadReplConfig(configPath, addr, fsl, clnt)
	// Reread until successful
	for err != nil {
		db.DLPrintf("RSRV", "Couldn't read repl config: %v\n", err)
		config, err = replchain.ReadReplConfig(configPath, addr, fsl, clnt)
	}
	config.UnionDirPath = unionDirPath
	config.SymlinkPath = symlinkPath
	return config
}
