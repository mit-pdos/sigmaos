package pyproxysrv

import (
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
	for {
		buf := make([]byte, 512)
		bytesRead, err := conn.Read(buf)
		if err != nil {
			db.DPrintf(db.PYPROXYSRV_ERR, "reader: rf err %v\n", err)
			return
		}

		data := buf[0:bytesRead]
		db.DPrintf(db.PYPROXYSRV, "reader: received %v\n", string(data))

		response := []byte("d\n")
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
