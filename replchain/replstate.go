package replchain

import (
	"log"
	"path"
	"sort"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fid"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
	"ulambda/protclnt"
)

const (
	MAX_CONNECT_RETRIES = 1000
)

type ReplState struct {
	mu         *sync.Mutex
	config     *NetServerReplConfig
	HeadChan   *RelayNetConn
	TailChan   *RelayNetConn
	PrevChan   *RelayNetConn
	NextChan   *RelayNetConn
	ops        chan *RelayOp
	inFlight   *RelayOpSet
	fids       map[np.Tfid]*fid.Fid
	replyCache *ReplyCache
	*fslib.FsLib
	proc.ProcClnt
	*protclnt.Clnt
}

func MakeReplState(c *NetServerReplConfig) *ReplState {
	config := CopyNetServerReplConfig(c)
	fsl := fslib.MakeFsLib("replstate")
	ops := make(chan *RelayOp)
	return &ReplState{&sync.Mutex{},
		config,
		nil, nil, nil, nil,
		ops,
		MakeRelayOpSet(),
		map[np.Tfid]*fid.Fid{},
		MakeReplyCache(),
		fsl,
		procinit.MakeProcClnt(fsl, procinit.GetProcLayersMap()),
		protclnt.MakeClnt(),
	}
}

func (rs *ReplState) Init() {
	// Set up op logging if necessary
	if rs.config.LogOps {
		err := rs.MakeFile("name/"+rs.config.RelayAddr+"-log.txt", 0777, np.OWRITE, []byte(""))
		if err != nil {
			log.Fatalf("Error making log file: %v", err)
		}
	}
	// Load the config & continuously watch for changes
	rs.reloadReplConfig(rs.config)
	go rs.runReplConfigUpdater()
	// Watch for other servers that go down, and spawn a lambda to update config
	go rs.runDirWatcher()
}

func (rs *ReplState) getNewReplConfig() *NetServerReplConfig {
	for {
		config, err := ReadReplConfig(rs.config.ConfigPath, rs.config.RelayAddr, rs.FsLib, rs.Clnt)
		if err != nil {
			if !strings.Contains(err.Error(), "file not found") {
				log.Printf("Error reading new config: %v, %v", rs.config.ConfigPath, err)
			}
			continue
		}
		return config
	}
}

// Updates addresses if any have changed, and connects to new peers.
func (rs *ReplState) reloadReplConfig(cfg *NetServerReplConfig) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.config.HeadAddr != cfg.HeadAddr || rs.HeadChan == nil {
		rs.connectToReplica(&rs.HeadChan, cfg.HeadAddr)
		rs.config.HeadAddr = cfg.HeadAddr
	}
	if rs.config.TailAddr != cfg.TailAddr || rs.TailChan == nil {
		rs.connectToReplica(&rs.TailChan, cfg.TailAddr)
		rs.config.TailAddr = cfg.TailAddr
	}
	if rs.config.PrevAddr != cfg.PrevAddr || rs.PrevChan == nil {
		rs.connectToReplica(&rs.PrevChan, cfg.PrevAddr)
		rs.config.PrevAddr = cfg.PrevAddr
	}
	if rs.config.NextAddr != cfg.NextAddr || rs.NextChan == nil {
		rs.connectToReplica(&rs.NextChan, cfg.NextAddr)
		rs.config.NextAddr = cfg.NextAddr
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

func (rs *ReplState) connectToReplica(rc **RelayNetConn, addr string) {
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
				log.Printf("Error connecting RelayConn: %v, %v", rs.config.RelayAddr, err)
			}
		} else {
			*rc = ch
			return
		}
	}
}

func (rs *ReplState) isHead() bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.config.RelayAddr == rs.config.HeadAddr
}

func (rs *ReplState) isTail() bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.config.RelayAddr == rs.config.TailAddr
}

// Watch in case servers go down, and start a lambda to update the config if
// they do.
func (rs *ReplState) runDirWatcher() {
	config := rs.config
	for {
		done := make(chan bool)
		rs.SetDirWatch(config.UnionDirPath, func(p string, err error) {
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
		rs.Spawn(attr)
	}
}

func (rs *ReplState) getReplicaTargets() []string {
	config := rs.config
	targets := []string{}
	// Get list of replica links
	d, err := rs.ReadDir(config.UnionDirPath)
	// Sort them
	sort.Slice(d, func(i, j int) bool {
		return d[i].Name < d[j].Name
	})
	if err != nil {
		log.Printf("Error getting replica targets: %v", err)
	}
	// Read links
	for _, replica := range d {
		target, err := rs.ReadFile(path.Join(rs.config.UnionDirPath, replica.Name))
		if err != nil {
			log.Printf("Error reading link file: %v", err)
			continue
		}
		targets = append(targets, string(target))
	}
	return targets
}

// Watch for changes to the config file, and update if necessary
func (rs *ReplState) runReplConfigUpdater() {
	for {
		done := make(chan bool)
		rs.SetRemoveWatch(rs.config.ConfigPath, func(p string, err error) {
			db.DLPrintf("RSRV", "%v detected new config\n", rs.config.RelayAddr)
			if err != nil && err.Error() == "EOF" {
				return
			} else if err != nil && !strings.Contains(err.Error(), "Version mismatch") {
				log.Printf("Error in runReplConfigUpdater RemoveWatch: %v", err)
			}
			done <- true
		})
		<-done
		config := rs.getNewReplConfig()
		db.DLPrintf("RSRV", "%v reloading config: %v\n", rs.config.RelayAddr, config)
		rs.reloadReplConfig(config)
		// If we are the head, write a symlink
		if rs.isHead() {
			targets := rs.getReplicaTargets()
			db.DLPrintf("RSRV", "%v has become the head. Creating symlink %v -> %v", rs.config.RelayAddr, rs.config.SymlinkPath, targets)
			rs.Remove(rs.config.SymlinkPath)
			rs.SymlinkReplica(targets, rs.config.SymlinkPath, 0777|np.DMTMP|np.DMREPL)
		}
		// Resend any in-flight messages. Do this asynchronously in case the sends
		// fail.
		go resendInflightRelayOps(rs)
		if rs.isTail() {
			db.DLPrintf("RSRV", "%v had become the tail in configUpdater", rs.config.RelayAddr)
			sendAllAcks(rs)
		}
	}
}
