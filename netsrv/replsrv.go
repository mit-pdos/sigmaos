package netsrv

import (
	"fmt"
	"log"
	"net"
	"path"
	"sort"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
	"ulambda/protclnt"
	"ulambda/protsrv"
)

const (
	MAX_CONNECT_RETRIES = 1000
)

type ServerReplConfig interface {
	Lock()
	Unlock()
}

type NetServerReplConfig struct {
	LogOps       bool
	ConfigPath   string
	UnionDirPath string
	SymlinkPath  string
	RelayAddr    string
	LastSendAddr string
	HeadAddr     string
	TailAddr     string
	PrevAddr     string
	NextAddr     string
}

func MakeReplicatedNetServer(fs protsrv.FsServer, address string, wireCompat bool, replicated bool, config *NetServerReplConfig) *NetServer {
	var emptyConfig *NetServerReplConfig
	var replState *ReplState
	if replicated {
		db.DLPrintf("RSRV", "starting replicated server: %v\n", config)
		replState = MakeReplState()
		emptyConfig = &NetServerReplConfig{
			config.LogOps,
			config.ConfigPath,
			config.UnionDirPath,
			config.SymlinkPath,
			config.RelayAddr,
			"", "", "", "", "",
		}
	}
	srv := &NetServer{"",
		fs,
		wireCompat, replicated,
		emptyConfig,
		replState,
	}
	if replicated {
		// Create and start the relay server listener
		db.DLPrintf("RSRV", "listen %v  myaddr %v\n", address, srv.addr)
		relayL, err := net.Listen("tcp", config.RelayAddr)
		if err != nil {
			log.Fatal("Relay listen error:", err)
		}
		// Set up op logging if necessary
		if config.LogOps {
			err = replState.MakeFile("name/"+config.RelayAddr+"-log.txt", 0777, np.OWRITE, []byte(""))
			if err != nil {
				log.Fatalf("Error making log file: %v", err)
			}
		}
		log.Printf("srv0 %v\n", srv.fssrv)
		// Start a server to listen for relay messages
		go srv.runsrv(relayL)
		// Load the config & continuously watch for changes
		srv.reloadReplConfig(config)
		go srv.runReplConfigUpdater()
		// Watch for other servers that go down, and spawn a lambda to update config
		go srv.runDirWatcher()
	}
	// Create and start the main server listener
	db.DLPrintf("9PCHAN", "listen %v  myaddr %v\n", address, srv.addr)
	var l net.Listener
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	srv.addr = l.Addr().String()
	go srv.runsrv(l)
	return srv
}

func (srv *NetServer) getNewReplConfig() *NetServerReplConfig {
	for {
		config, err := ReadReplConfig(srv.replConfig.ConfigPath, srv.replConfig.RelayAddr, srv.replState.FsLib, srv.replState.Clnt)
		if err != nil {
			if !strings.Contains(err.Error(), "file not found") {
				log.Printf("Error reading new config: %v, %v", srv.replConfig.ConfigPath, err)
			}
			continue
		}
		return config
	}
}

// Updates addresses if any have changed, and connects to new peers.
func (srv *NetServer) reloadReplConfig(cfg *NetServerReplConfig) {
	srv.replState.mu.Lock()
	defer srv.replState.mu.Unlock()
	if srv.replConfig.HeadAddr != cfg.HeadAddr || srv.replState.HeadChan == nil {
		srv.connectToReplica(&srv.replState.HeadChan, cfg.HeadAddr)
		srv.replConfig.HeadAddr = cfg.HeadAddr
	}
	if srv.replConfig.TailAddr != cfg.TailAddr || srv.replState.TailChan == nil {
		srv.connectToReplica(&srv.replState.TailChan, cfg.TailAddr)
		srv.replConfig.TailAddr = cfg.TailAddr
	}
	if srv.replConfig.PrevAddr != cfg.PrevAddr || srv.replState.PrevChan == nil {
		srv.connectToReplica(&srv.replState.PrevChan, cfg.PrevAddr)
		srv.replConfig.PrevAddr = cfg.PrevAddr
	}
	if srv.replConfig.NextAddr != cfg.NextAddr || srv.replState.NextChan == nil {
		srv.connectToReplica(&srv.replState.NextChan, cfg.NextAddr)
		srv.replConfig.NextAddr = cfg.NextAddr
	}
}

// Read a replication config file.
func ReadReplConfig(path string, myaddr string, fsl *fslib.FsLib, clnt *protclnt.Clnt) (*NetServerReplConfig, error) {
	b, err := fsl.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfgString := strings.TrimSpace(string(b))
	servers := strings.Split(cfgString, "\n")
	headAddr := servers[0]
	tailAddr := servers[len(servers)-1]
	prevAddr := tailAddr
	nextAddr := headAddr
	for idx, s := range servers {
		if s == myaddr {
			if idx != 0 {
				prevAddr = servers[idx-1]
			}
			if idx != len(servers)-1 {
				nextAddr = servers[idx+1]
			}
		}
	}
	return &NetServerReplConfig{
		false,
		path,
		"",
		"",
		myaddr,
		"",
		headAddr, tailAddr, prevAddr, nextAddr,
	}, nil
}

func (srv *NetServer) connectToReplica(rc **RelayNetConn, addr string) {
	// If there was an old channel here, close it.
	if *rc != nil {
		(*rc).Close()
	}
	if addr == "" {
		return
	}
	for i := 0; i < MAX_CONNECT_RETRIES; i++ {
		// May need to retry if receiving server hasn't started up yet.
		ch, err := MakeRelayNetConn(addr)
		if err != nil {
			if !strings.Contains(err.Error(), "connection refused") && !peerCrashed(err) {
				log.Printf("Error connecting RelayConn: %v, %v", srv.addr, err)
			}
		} else {
			*rc = ch
			return
		}
	}
}

func (srv *NetServer) isHead() bool {
	srv.replState.mu.Lock()
	defer srv.replState.mu.Unlock()
	return srv.replConfig.RelayAddr == srv.replConfig.HeadAddr
}

func (srv *NetServer) isTail() bool {
	srv.replState.mu.Lock()
	defer srv.replState.mu.Unlock()
	return srv.replConfig.RelayAddr == srv.replConfig.TailAddr
}

// Watch in case servers go down, and start a lambda to update the config if
// they do.
func (srv *NetServer) runDirWatcher() {
	config := srv.replConfig
	for {
		done := make(chan bool)
		srv.replState.SetDirWatch(config.UnionDirPath, func(p string, err error) {
			db.DLPrintf("RSRV", "%v Dir watch triggered!", config.RelayAddr)
			if err != nil && err.Error() == "EOF" {
				return
			} else if err != nil && !strings.Contains(err.Error(), "Version mismatch") {
				log.Printf("Error in ReplicaMonitor DirWatch: %v", err)
			}
			done <- true
		})
		<-done
		attr := &proc.Proc{}
		attr.Pid = proc.GenPid()
		attr.Program = "bin/user/replica-monitor"
		attr.Args = []string{config.ConfigPath, config.UnionDirPath}
		attr.Env = []string{procinit.GetProcLayersString()}
		srv.replState.Spawn(attr)
	}
}

func (srv *NetServer) getReplicaTargets() []string {
	config := srv.replConfig
	targets := []string{}
	// Get list of replica links
	d, err := srv.replState.ReadDir(config.UnionDirPath)
	// Sort them
	sort.Slice(d, func(i, j int) bool {
		return d[i].Name < d[j].Name
	})
	if err != nil {
		log.Printf("Error getting replica targets: %v", err)
	}
	// Read links
	for _, replica := range d {
		target, err := srv.replState.ReadFile(path.Join(srv.replConfig.UnionDirPath, replica.Name))
		if err != nil {
			log.Printf("Error reading link file: %v", err)
			continue
		}
		targets = append(targets, string(target))
	}
	return targets
}

// Watch for changes to the config file, and update if necessary
func (srv *NetServer) runReplConfigUpdater() {
	for {
		done := make(chan bool)
		srv.replState.SetRemoveWatch(srv.replConfig.ConfigPath, func(p string, err error) {
			db.DLPrintf("RSRV", "%v detected new config\n", srv.replConfig.RelayAddr)
			if err != nil && err.Error() == "EOF" {
				return
			} else if err != nil && !strings.Contains(err.Error(), "Version mismatch") {
				log.Printf("Error in runReplConfigUpdater RemoveWatch: %v", err)
			}
			done <- true
		})
		<-done
		config := srv.getNewReplConfig()
		db.DLPrintf("RSRV", "%v reloading config: %v\n", srv.replConfig.RelayAddr, config)
		srv.reloadReplConfig(config)
		// If we are the head, write a symlink
		if srv.isHead() {
			targets := srv.getReplicaTargets()
			db.DLPrintf("RSRV", "%v has become the head. Creating symlink %v -> %v", srv.replConfig.RelayAddr, srv.replConfig.SymlinkPath, targets)
			srv.replState.Remove(srv.replConfig.SymlinkPath)
			srv.replState.SymlinkReplica(targets, srv.replConfig.SymlinkPath, 0777|np.DMTMP|np.DMREPL)
		}
		// Resend any in-flight messages. Do this asynchronously in case the sends
		// fail.
		go resendInflightRelayOps(srv)
		if srv.isTail() {
			db.DLPrintf("RSRV", "%v had become the tail in configUpdater", srv.replConfig.RelayAddr)
			sendAllAcks(srv)
		}
	}
}

func (c *NetServerReplConfig) String() string {
	return fmt.Sprintf("{ relayAddr: %v head: %v tail: %v prev: %v next: %v }", c.RelayAddr, c.HeadAddr, c.TailAddr, c.PrevAddr, c.NextAddr)
}
