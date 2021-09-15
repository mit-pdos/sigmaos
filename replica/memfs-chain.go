package replica

import (
	"log"
	"path"

	db "ulambda/debug"
	"ulambda/fslibsrv"
	"ulambda/memfsd"
	"ulambda/proc"
	"ulambda/procinit"
	"ulambda/repl"
)

type MemfsdReplica struct {
	Pid    string
	name   string
	config repl.Config
	fsd    *memfsd.Fsd
	*fslibsrv.FsLibSrv
	proc.ProcClnt
}

func MakeMemfsdReplica(args []string, srvAddr string, unionDirPath string, config repl.Config) *MemfsdReplica {
	r := &MemfsdReplica{}
	r.Pid = args[0]
	r.config = config
	//	r.relayPort = args[1]
	//	portNum, err := strconv.Atoi(r.relayPort)
	//	if err != nil {
	//		log.Fatalf("Relay port must be an integer")
	//	}
	//
	//	// Server port is relay port + 100
	//	r.srvPort = strconv.Itoa(100 + portNum)
	//	r.configPath = args[2]
	//	r.unionDirPath = args[3]
	//	r.symlinkPath = args[4]
	//	ip, err := fsclnt.LocalIP()
	//	if err != nil {
	//		log.Fatalf("%v: no IP %v\n", args, err)
	//	}
	//	r.relayAddr = ip + ":" + r.relayPort
	//	r.srvAddr = ip + ":" + r.srvPort
	//	r.config = getConfig(r)
	//	if len(args) == 6 && args[5] == "log-ops" {
	//		r.config.LogOps = true
	//	}
	r.fsd = memfsd.MakeReplicatedFsd(srvAddr, true, r.config)
	r.name = path.Join(unionDirPath, config.ReplAddr())
	db.Name(r.name)
	fs, err := fslibsrv.InitFs(r.name, r.fsd, nil)
	if err != nil {
		log.Fatalf("%v: InitFs failed %v\n", args, err)
	}
	r.FsLibSrv = fs
	r.ProcClnt = procinit.MakeProcClnt(fs.FsLib, procinit.GetProcLayersMap())
	return r
}

func (r *MemfsdReplica) Work() {
	r.fsd.Serve()
	r.ExitFs(r.name)
}

//func (r *MemfsdReplica) GetAddr() string {
//	return r.relayAddr
//}
//
//func (r *MemfsdReplica) GetPort() string {
//	return r.relayPort
//}
//
//func (r *MemfsdReplica) GetConfigPath() string {
//	return r.configPath
//}
//
//func (r *MemfsdReplica) GetUnionDirPath() string {
//	return r.unionDirPath
//}
//
//func (r *MemfsdReplica) GetServiceName() string {
//	return "memfsd"
//}
//
//func (r *MemfsdReplica) GetSymlinkPath() string {
//	return r.symlinkPath
//}
