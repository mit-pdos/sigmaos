package npsrv

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/npclnt"
	"ulambda/npobjsrv"
)

const (
	MAX_CONNECT_RETRIES = 1000
)

type NpServerReplConfig struct {
	mu           sync.Mutex
	LogOps       bool
	ConfigPath   string
	UnionDirPath string
	RelayAddr    string
	LastSendAddr string
	HeadAddr     string
	TailAddr     string
	PrevAddr     string
	NextAddr     string
	HeadChan     *RelayConn
	TailChan     *RelayConn
	PrevChan     *RelayConn
	NextChan     *RelayConn
	ops          chan *SrvOp
	q            *RelayMsgQueue
	fids         map[np.Tfid]*npobjsrv.Fid
	*fslib.FsLib
	*npclnt.NpClnt
}

func MakeReplicatedNpServer(npc NpConn, address string, wireCompat bool, replicated bool, relayAddr string, config *NpServerReplConfig) *NpServer {
	var emptyConfig *NpServerReplConfig
	if replicated {
		db.DLPrintf("RSRV", "starting replicated server: %v\n", config)
		ops := make(chan *SrvOp)
		emptyConfig = &NpServerReplConfig{sync.Mutex{},
			config.LogOps,
			config.ConfigPath,
			config.UnionDirPath,
			relayAddr,
			"", "", "", "", "",
			nil, nil, nil, nil,
			ops,
			&RelayMsgQueue{},
			map[np.Tfid]*npobjsrv.Fid{},
			config.FsLib,
			config.NpClnt}
	}
	srv := &NpServer{npc, "", wireCompat, replicated, emptyConfig}
	var l net.Listener
	if replicated {
		registerGobTypes()
		// Create and start the relay server listener
		db.DLPrintf("RSRV", "listen %v  myaddr %v\n", address, srv.addr)
		relayL, err := net.Listen("tcp", relayAddr)
		if err != nil {
			log.Fatal("Relay listen error:", err)
		}
		// Set up op logging if necessary
		if config.LogOps {
			err = config.MakeFile("name/"+relayAddr+"-log.txt", 0777, []byte(""))
			if err != nil {
				log.Fatalf("Error making log file: %v", err)
			}
		}
		// Start a server to listen for relay messages
		go srv.runsrv(relayL, true)
		// Load the config & continuously watch for changes
		srv.reloadReplConfig(config)
		go srv.runReplConfigUpdater()
		// Watch for other servers that go down, and spawn a lambda to update config
		go srv.runDirWatcher()
		// Set up the relay for this server
		srv.setupRelay()
	}
	// Create and start the main server listener
	db.DLPrintf("9PCHAN", "listen %v  myaddr %v\n", address, srv.addr)
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	srv.addr = l.Addr().String()
	go srv.runsrv(l, false)
	return srv
}

func (srv *NpServer) getNewReplConfig() *NpServerReplConfig {
	for {
		config, err := ReadReplConfig(srv.replConfig.ConfigPath, srv.replConfig.RelayAddr, srv.replConfig.FsLib, srv.replConfig.NpClnt)
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
func (srv *NpServer) reloadReplConfig(cfg *NpServerReplConfig) {
	srv.replConfig.mu.Lock()
	defer srv.replConfig.mu.Unlock()
	if srv.replConfig.HeadAddr != cfg.HeadAddr || srv.replConfig.HeadChan == nil {
		srv.connectToReplica(&srv.replConfig.HeadChan, cfg.HeadAddr)
		srv.replConfig.HeadAddr = cfg.HeadAddr
	}
	if srv.replConfig.TailAddr != cfg.TailAddr || srv.replConfig.TailChan == nil {
		srv.connectToReplica(&srv.replConfig.TailChan, cfg.TailAddr)
		srv.replConfig.TailAddr = cfg.TailAddr
	}
	if srv.replConfig.PrevAddr != cfg.PrevAddr || srv.replConfig.PrevChan == nil {
		srv.connectToReplica(&srv.replConfig.PrevChan, cfg.PrevAddr)
		srv.replConfig.PrevAddr = cfg.PrevAddr
	}
	if srv.replConfig.NextAddr != cfg.NextAddr || srv.replConfig.NextChan == nil {
		srv.connectToReplica(&srv.replConfig.NextChan, cfg.NextAddr)
		srv.replConfig.NextAddr = cfg.NextAddr
	}
}

// Read a replication config file.
func ReadReplConfig(path string, myaddr string, fsl *fslib.FsLib, clnt *npclnt.NpClnt) (*NpServerReplConfig, error) {
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
	return &NpServerReplConfig{sync.Mutex{},
		false,
		path,
		"",
		myaddr,
		"",
		headAddr, tailAddr, prevAddr, nextAddr,
		nil, nil, nil, nil,
		nil,
		nil,
		nil,
		fsl,
		clnt}, nil
}

func (srv *NpServer) connectToReplica(rc **RelayConn, addr string) {
	// If there was an old channel here, close it.
	if *rc != nil {
		(*rc).Close()
	}
	if addr == "" {
		return
	}
	for i := 0; i < MAX_CONNECT_RETRIES; i++ {
		// May need to retry if receiving server hasn't started up yet.
		ch, err := MakeRelayConn(addr)
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

func (srv *NpServer) isHead() bool {
	srv.replConfig.mu.Lock()
	defer srv.replConfig.mu.Unlock()
	return srv.replConfig.RelayAddr == srv.replConfig.HeadAddr
}

func (srv *NpServer) isTail() bool {
	srv.replConfig.mu.Lock()
	defer srv.replConfig.mu.Unlock()
	return srv.replConfig.RelayAddr == srv.replConfig.TailAddr
}

// Watch in case servers go down, and start a lambda to update the config if
// they do.
func (srv *NpServer) runDirWatcher() {
	config := srv.replConfig
	for {
		done := make(chan bool)
		config.SetDirWatch(config.UnionDirPath, func(p string, err error) {
			db.DLPrintf("RSRV", "%v Dir watch triggered!", config.RelayAddr)
			if err != nil && err.Error() == "EOF" {
				return
			} else if err != nil && !strings.Contains(err.Error(), "Version mismatch") {
				log.Printf("Error in ReplicaMonitor DirWatch: %v", err)
			}
			done <- true
		})
		<-done
		attr := &fslib.Attr{}
		attr.Pid = fslib.GenPid()
		attr.Program = "bin/replica-monitor"
		attr.Args = []string{config.ConfigPath, config.UnionDirPath}
		config.Spawn(attr)
	}
}

// Watch for changes to the config file, and update if necessary
func (srv *NpServer) runReplConfigUpdater() {
	for {
		done := make(chan bool)
		srv.replConfig.SetRemoveWatch(srv.replConfig.ConfigPath, func(p string, err error) {
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
		// Resend any in-flight messages. Do this asynchronously in case the sends
		// fail.
		go srv.resendInflightRelayMsgs()
		if srv.isTail() {
			db.DLPrintf("RSRV", "%v had become the tail in configUpdater", srv.replConfig.RelayAddr)
			srv.sendAllAcks()
		}
	}
}

func (c *NpServerReplConfig) String() string {
	return fmt.Sprintf("{ relayAddr: %v head: %v tail: %v prev: %v next: %v }", c.RelayAddr, c.HeadAddr, c.TailAddr, c.PrevAddr, c.NextAddr)
}

func registerGobTypes() {
	gob.Register(np.Tattach{})
	gob.Register(np.Rattach{})
	gob.Register(np.Tversion{})
	gob.Register(np.Rversion{})
	gob.Register(np.Tauth{})
	gob.Register(np.Rauth{})
	gob.Register(np.Tattach{})
	gob.Register(np.Rattach{})
	gob.Register(np.Rerror{})
	gob.Register(np.Tflush{})
	gob.Register(np.Rflush{})
	gob.Register(np.Twalk{})
	gob.Register(np.Rwalk{})
	gob.Register(np.Topen{})
	gob.Register(np.Ropen{})
	gob.Register(np.Tcreate{})
	gob.Register(np.Rcreate{})
	gob.Register(np.Tread{})
	gob.Register(np.Rread{})
	gob.Register(np.Twrite{})
	gob.Register(np.Rwrite{})
	gob.Register(np.Tclunk{})
	gob.Register(np.Rclunk{})
	gob.Register(np.Tremove{})
	gob.Register(np.Rremove{})
	gob.Register(np.Tstat{})
	gob.Register(np.Rstat{})
	gob.Register(np.Twstat{})
	gob.Register(np.Rwstat{})
	gob.Register(np.Treadv{})
	gob.Register(np.Twritev{})
	gob.Register(np.Twatchv{})
	gob.Register(np.Trenameat{})
	gob.Register(np.Rrenameat{})
}
