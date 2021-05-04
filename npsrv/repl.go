package npsrv

import (
	"log"
	"net"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/npclnt"
)

type NpServerReplConfig struct {
	Path     string
	HeadAddr string
	TailAddr string
	PrevAddr string
	NextAddr string
	HeadChan *npclnt.NpChan
	TailChan *npclnt.NpChan
	PrevChan *npclnt.NpChan
	NextChan *npclnt.NpChan
	ops      chan *RelayOp
	*fslib.FsLib
	*npclnt.NpClnt
}

func MakeReplicatedNpServer(npc NpConn, address string, replicated bool, config *NpServerReplConfig) *NpServer {
	var emptyConfig *NpServerReplConfig
	if replicated {
		db.DLPrintf("9PSRV", "starting replicated server: %v\n", config)
		ops := make(chan *RelayOp)
		emptyConfig = &NpServerReplConfig{config.Path, "", "", "", "", nil, nil, nil, nil, ops, config.FsLib, config.NpClnt}
	}
	srv := &NpServer{npc, "", replicated, emptyConfig}
	var l net.Listener
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	srv.addr = l.Addr().String()
	if replicated {
		srv.reloadReplConfig(config)
		go srv.runReplConfigUpdater()
		go srv.relayWorker()
	}
	db.DLPrintf("9PCHAN", "listen %v  myaddr %v\n", address, srv.addr)
	go srv.runsrv(l)
	return srv
}

func (srv *NpServer) getNewReplConfig() *NpServerReplConfig {
	for {
		config, err := ReadReplConfig(srv.replConfig.Path, srv.addr, srv.replConfig.FsLib, srv.replConfig.NpClnt)
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
	// TODO locking, trigger resends, etc.
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
	return &NpServerReplConfig{path, headAddr, tailAddr, prevAddr, nextAddr, nil, nil, nil, nil, nil, fsl, clnt}, nil
}

func (srv *NpServer) connectToReplica(c **npclnt.NpChan, addr string) {
	*c = srv.replConfig.MakeNpChan(addr)
}

func (srv *NpServer) isHead() bool {
	return srv.addr == srv.replConfig.HeadAddr
}

func (srv *NpServer) isTail() bool {
	return srv.addr == srv.replConfig.TailAddr
}

func (srv *NpServer) runReplConfigUpdater() {
	for {
		done := make(chan bool)
		srv.replConfig.SetRemoveWatch(srv.replConfig.Path, func(p string, err error) {
			log.Printf("Srv %v detected new config!", srv.addr)
			if err != nil && err.Error() == "EOF" {
				return
			} else if err != nil {
				log.Printf("Error in runReplConfigUpdater RemoveWatch: %v", err)
			}
			done <- true
		})
		<-done
		config := srv.getNewReplConfig()
		srv.reloadReplConfig(config)
	}
}
