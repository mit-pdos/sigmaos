package replica

import (
	"log"
	"os"
	"path"
	"strconv"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/fsux"
	"ulambda/proc"
	"ulambda/procinit"
	"ulambda/replchain"
)

type FsUxReplica struct {
	Pid          string
	name         string
	relayPort    string
	relayAddr    string
	srvPort      string
	srvAddr      string
	configPath   string
	unionDirPath string
	symlinkPath  string
	mount        string
	config       *replchain.NetServerReplConfig
	ux           *fsux.FsUx
	*fslib.FsLib
	proc.ProcClnt
}

func MakeFsUxReplica(args []string) *FsUxReplica {
	r := &FsUxReplica{}
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
	r.symlinkPath = args[4]
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("%v: no IP %v\n", args, err)
	}
	r.relayAddr = ip + ":" + r.relayPort
	r.srvAddr = ip + ":" + r.srvPort
	r.config = getConfig(r)
	if len(args) == 6 && args[5] == "log-ops" {
		r.config.LogOps = true
	}
	fsl := fslib.MakeFsLib("fsux-replica" + r.relayAddr)
	r.FsLib = fsl
	r.ProcClnt = procinit.MakeProcClnt(fsl, procinit.GetProcLayersMap())
	r.mount = "/tmp"
	r.ux = fsux.MakeReplicatedFsUx(r.mount, r.srvAddr, "", true, r.config)
	r.name = path.Join(r.unionDirPath, r.relayAddr)
	// Post in union dir
	err = r.PostService(r.srvAddr, r.name)
	if err != nil {
		log.Fatalf("PostService %v error: %v", r.name, err)
	}
	db.Name(r.name)
	//	fs, err := fslibsrv.InitFs(r.name, r.ux, nil)
	//	if err != nil {
	//		log.Fatalf("%v: InitFs failed %v\n", args, err)
	//	}
	//	r.FsLibSrv = fs
	return r
}

func (r *FsUxReplica) setupMountPoint() {
	r.mount = "/tmp/fsux-" + r.relayAddr
	// Remove the old mount if it already existed
	os.RemoveAll(r.mount)
	os.Mkdir(r.mount, 0777)
}

func (r *FsUxReplica) Work() {
	r.ux.Serve()
}

func (r *FsUxReplica) GetAddr() string {
	return r.relayAddr
}

func (r *FsUxReplica) GetPort() string {
	return r.relayPort
}

func (r *FsUxReplica) GetConfigPath() string {
	return r.configPath
}

func (r *FsUxReplica) GetUnionDirPath() string {
	return r.unionDirPath
}

func (r *FsUxReplica) GetServiceName() string {
	return "fsux"
}

func (r *FsUxReplica) GetSymlinkPath() string {
	return r.symlinkPath
}
