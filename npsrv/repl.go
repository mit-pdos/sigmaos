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
)

type NpServerReplConfig struct {
	mu        sync.Mutex
	Path      string
	RelayAddr string
	HeadAddr  string
	TailAddr  string
	PrevAddr  string
	NextAddr  string
	HeadChan  *RelayChan
	TailChan  *RelayChan
	PrevChan  *RelayChan
	NextChan  *RelayChan
	ops       chan *RelayOp
	*fslib.FsLib
	*npclnt.NpClnt
}

func MakeReplicatedNpServer(npc NpConn, address string, replicated bool, relayAddr string, config *NpServerReplConfig) *NpServer {
	var emptyConfig *NpServerReplConfig
	if replicated {
		db.DLPrintf("9PSRV", "starting replicated server: %v\n", config)
		ops := make(chan *RelayOp)
		emptyConfig = &NpServerReplConfig{sync.Mutex{}, config.Path, relayAddr, "", "", "", "", nil, nil, nil, nil, ops, config.FsLib, config.NpClnt}
	}
	srv := &NpServer{npc, "", replicated, emptyConfig}
	var l net.Listener
	if replicated {
		registerGobTypes()
		// Create and start the relay server listener
		db.DLPrintf("9PCHAN", "listen %v  myaddr %v\n", address, srv.addr)
		relayL, err := net.Listen("tcp", relayAddr)
		if err != nil {
			log.Fatal("Relay listen error:", err)
		}
		srv.addr = relayL.Addr().String()
		go srv.runsrv(relayL, true)
		srv.reloadReplConfig(config)
		go srv.runReplConfigUpdater()
		go srv.relayChanWorker()
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
		config, err := ReadReplConfig(srv.replConfig.Path, srv.replConfig.RelayAddr, srv.replConfig.FsLib, srv.replConfig.NpClnt)
		if err != nil {
			if strings.Index(err.Error(), "file not found") != 0 {
				log.Printf("Error reading new config: %v, %v", srv.replConfig.Path, err)
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
	if srv.replConfig.HeadAddr != cfg.HeadAddr {
		srv.connectToReplica(&srv.replConfig.HeadChan, cfg.HeadAddr)
		srv.replConfig.HeadAddr = cfg.HeadAddr
	}
	if srv.replConfig.TailAddr != cfg.TailAddr {
		srv.connectToReplica(&srv.replConfig.TailChan, cfg.TailAddr)
		srv.replConfig.TailAddr = cfg.TailAddr
	}
	if srv.replConfig.PrevAddr != cfg.PrevAddr {
		srv.connectToReplica(&srv.replConfig.PrevChan, cfg.PrevAddr)
		srv.replConfig.PrevAddr = cfg.PrevAddr
	}
	if srv.replConfig.NextAddr != cfg.NextAddr {
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
	return &NpServerReplConfig{sync.Mutex{}, path, myaddr, headAddr, tailAddr, prevAddr, nextAddr, nil, nil, nil, nil, nil, fsl, clnt}, nil
}

func (srv *NpServer) connectToReplica(rc **RelayChan, addr string) {
	// If there was an old channel here, close it.
	if *rc != nil {
		(*rc).Close()
	}
	for {
		// May need to retry if receiving server hasn't started up yet.
		ch, err := MakeRelayChan(addr)
		if err != nil {
			if strings.Index(err.Error(), "connection refused") == -1 {
				log.Printf("Error connecting RelayChan: %v, %v", srv.addr, err)
			}
		} else {
			*rc = ch
			break
		}
	}
}

func (srv *NpServer) isHead() bool {
	return srv.replConfig.RelayAddr == srv.replConfig.HeadAddr
}

func (srv *NpServer) isTail() bool {
	return srv.replConfig.RelayAddr == srv.replConfig.TailAddr
}

func (srv *NpServer) runReplConfigUpdater() {
	for {
		done := make(chan bool)
		srv.replConfig.SetRemoveWatch(srv.replConfig.Path, func(p string, err error) {
			log.Printf("Srv %v detected new config!", srv.replConfig.RelayAddr)
			if err != nil && err.Error() == "EOF" {
				return
			} else if err != nil {
				log.Printf("Error in runReplConfigUpdater RemoveWatch: %v", err)
			}
			done <- true
		})
		<-done
		config := srv.getNewReplConfig()
		log.Printf("%v reloading config: %v", srv.replConfig.RelayAddr, config)
		srv.reloadReplConfig(config)
	}
}

func (c *NpServerReplConfig) String() string {
	return fmt.Sprintf("{ relayAddr: %v head: %v tail: %v prev: %v next: %v}", c.RelayAddr, c.HeadAddr, c.TailAddr, c.PrevAddr, c.NextAddr)
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
