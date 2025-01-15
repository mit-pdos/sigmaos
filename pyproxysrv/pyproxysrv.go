package pyproxysrv

import (
	"bufio"
	"net"
	"os"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// PyProxySrv maintains the state of the pyproxysrv.
type PyProxySrv struct {
	pe *proc.ProcEnv
	sc *sigmaclnt.SigmaClnt
}

func NewPyProxySrv(pe *proc.ProcEnv) (*PyProxySrv, error) {
	// Create the proxy socket
	socket, err := net.Listen("unix", sp.SIGMA_PYPROXY_SOCKET)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(sp.SIGMA_PYPROXY_SOCKET, 0777); err != nil {
		db.DFatalf("Err chmod sigmasocket: %v", err)
	}
	db.DPrintf(db.TEST, "runServer: pyproxysrv listening on %v", sp.SIGMA_PYPROXY_SOCKET)

	pps := &PyProxySrv{
		pe: pe,
	}
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return nil, err
	}
	pps.sc = sc

	go pps.runServer(socket)

	return pps, nil
}

func (pps *PyProxySrv) Shutdown() {
	db.DPrintf(db.PYPROXYSRV, "Shutdown")
	os.Remove(sp.SIGMA_PYPROXY_SOCKET)
}

func (pps *PyProxySrv) handleNewConn(conn *net.UnixConn) {
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			db.DPrintf(db.PYPROXYSRV_ERR, "reader: rf err %v\n", err)
			return
		}

		// if line[:len(line)-1] == "pb" {
		// 	db.DPrintf(db.PYPROXYSRV, "reader: received initialization request\n", err)

		// 	os.Chmod("/tmp/python", 0777)

		// 	err = os.Mkdir("/tmp/python/superlib", 0755)
		// 	if err != nil {
		// 		db.DPrintf(db.PYPROXYSRV_ERR, "reader: mkdir err %v\n", err)
		// 		return
		// 	}
		// }

		db.DPrintf(db.PYPROXYSRV, "reader: received %v", line)

		response := []byte("d")
		_, err = conn.Write(response)
		if err != nil {
			db.DPrintf(db.PYPROXYSRV_ERR, "reader: wf err %v\n", err)
			return
		}
	}
}

func (pps *PyProxySrv) runServer(l net.Listener) {
	db.DPrintf(db.PYPROXYSRV, "pyproxysrv running")
	for {
		conn, err := l.Accept()
		if err != nil {
			db.DFatalf("Error pyproxysrv Accept: %v", err)
			return
		}
		// Handle incoming connection
		go pps.handleNewConn(conn.(*net.UnixConn))
	}
}
