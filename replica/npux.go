package replica

import (
	"log"
	"path"
	"strconv"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslibsrv"
	"ulambda/memfsd"
	"ulambda/npsrv"
)

type NpuxReplica struct {
	Pid          string
	name         string
	relayPort    string
	relayAddr    string
	srvPort      string
	srvAddr      string
	configPath   string
	unionDirPath string
	config       *npsrv.NpServerReplConfig
	fsd          *memfsd.Fsd
	*fslibsrv.FsLibSrv
}

func MakeNpuxReplica(args []string) *NpuxReplica {
	r := &NpuxReplica{}
	r.Pid = args[0]
	r.relayPort = args[1]
	portNum, err := strconv.Atoi(r.relayPort)
	if err != nil {
		log.Fatalf("Relay port must be an integer")
	}
	// Server port is relay port + 100
	r.srvPort = strconv.Itoa(100 + portNum)
	r.configPath = args[2]
	r.unionDirPath = args[3]
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("%v: no IP %v\n", args, err)
	}
	r.relayAddr = ip + ":" + r.relayPort
	r.srvAddr = ip + ":" + r.srvPort
	r.config = getConfig(r)
	if len(args) == 5 && args[4] == "log-ops" {
		r.config.LogOps = true
	}
	r.fsd = memfsd.MakeReplicatedFsd(r.srvAddr, true, r.relayAddr, r.config)
	r.name = path.Join(r.unionDirPath, r.relayAddr)
	db.Name(r.name)
	fs, err := fslibsrv.InitFs(r.name, r.fsd, nil)
	if err != nil {
		log.Fatalf("%v: InitFs failed %v\n", args, err)
	}
	r.FsLibSrv = fs
	return r
}

func (r *NpuxReplica) Work() {
	r.Started(r.Pid)
	r.fsd.Serve()
	r.ExitFs(r.name)
}

func (r *NpuxReplica) GetAddr() string {
	return r.relayAddr
}

func (r *NpuxReplica) GetPort() string {
	return r.relayPort
}

func (r *NpuxReplica) GetConfigPath() string {
	return r.configPath
}

func (r *NpuxReplica) GetUnionDirPath() string {
	return r.unionDirPath
}

func (r *NpuxReplica) GetServiceName() string {
	return "npux"
}
