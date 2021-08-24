package replica

import (
	"log"
	"os"
	"path"
	"strconv"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/npsrv"
	"ulambda/npux"
	"ulambda/proc"
)

type NpUxReplica struct {
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
	config       *npsrv.NpServerReplConfig
	ux           *npux.NpUx
	*fslib.FsLib
	*proc.ProcCtl
}

func MakeNpUxReplica(args []string) *NpUxReplica {
	r := &NpUxReplica{}
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
	fsl := fslib.MakeFsLib("npux-replica" + r.relayAddr)
	r.FsLib = fsl
	r.ProcCtl = proc.MakeProcCtl(fsl)
	r.ux = npux.MakeReplicatedNpUx(r.mount, r.srvAddr, "", true, r.relayAddr, r.config)
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

func (r *NpUxReplica) setupMountPoint() {
	r.mount = "/tmp/npux-" + r.relayAddr
	// Remove the old mount if it already existed
	os.RemoveAll(r.mount)
	os.Mkdir(r.mount, 0777)
}

func (r *NpUxReplica) Work() {
	r.Started(r.Pid)
	r.ux.Serve()
}

func (r *NpUxReplica) GetAddr() string {
	return r.relayAddr
}

func (r *NpUxReplica) GetPort() string {
	return r.relayPort
}

func (r *NpUxReplica) GetConfigPath() string {
	return r.configPath
}

func (r *NpUxReplica) GetUnionDirPath() string {
	return r.unionDirPath
}

func (r *NpUxReplica) GetServiceName() string {
	return "npux"
}

func (r *NpUxReplica) GetSymlinkPath() string {
	return r.symlinkPath
}
