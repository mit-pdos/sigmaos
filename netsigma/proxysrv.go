package netsigma

import (
	"net"
	"os"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/frame"
	sp "sigmaos/sigmap"
)

type NetProxySrv struct {
	directDialFn DialFn
}

func NewNetProxySrv() *NetProxySrv {
	nps := &NetProxySrv{
		directDialFn: DialDirect,
	}
	go nps.runServer()
	return nps
}

func (nps *NetProxySrv) runServer() error {
	// Create the net proxy socket
	socket, err := net.Listen("unix", sp.SIGMA_NETPROXY_SOCKET)
	if err != nil {
		return err
	}
	if err := os.Chmod(sp.SIGMA_NETPROXY_SOCKET, 0777); err != nil {
		db.DFatalf("Err chmod sigmasocket: %v", err)
	}
	db.DPrintf(db.TEST, "runServer: netproxysrv listening on %v", sp.SIGMA_NETPROXY_SOCKET)

	for {
		conn, err := socket.Accept()
		if err != nil {
			db.DFatalf("Error netproxysrv Accept: %v", err)
			return err
		}
		// Handle incoming connection
		go nps.handleNewConn(conn.(*net.UnixConn))
	}
}

// TODO XXX will this leak goroutines?
func (nps *NetProxySrv) handleNewConn(conn *net.UnixConn) {
	defer conn.Close()
	for {
		b, err := frame.ReadFrame(conn)
		if err != nil {
			db.DPrintf(db.ERROR, "Error ReadFrame: %v", err)
			return
		}
		// Unmarshal the requested addr to dial
		addr := sp.UnmarshalTaddr(string(b))
		proxyConn, error := nps.directDialFn(addr)
		// TODO: handle dialing errors properly (by responding to the client). This
		// package should probably have its own proto pkg (and corresponding codec)
		if error != nil {
			db.DPrintf(db.ERROR, "Error dial direct: %v", error)
			return
		}
		// Send the newly established connection's FD back to the client
		error = sendProxiedConnSocketFD(conn, proxyConn)
		if err != nil {
			db.DPrintf(db.ERROR, "Error send conn FD: %v", error)
			return
		}
	}
}

func connToFD(proxyConn net.Conn) (int, error) {
	f, err := proxyConn.(*net.TCPConn).File()
	if err != nil {
		db.DFatalf("Error get unix conn fd: %v", err)
		return 0, err
	}
	// Return the unix FD for the socket
	return int(f.Fd()), nil
}

// Send the FD corresponding to the socket of the established (proxied)
// connection to the client.
func sendProxiedConnSocketFD(clntConn *net.UnixConn, proxyConn net.Conn) error {
	// Get the unix FD for the socket
	proxyConnFD, err := connToFD(proxyConn)
	if err != nil {
		db.DFatalf("Error get unix conn fd: %v", err)
		return err
	}
	oob := unix.UnixRights(proxyConnFD)
	db.DPrintf(db.NETPROXYSRV, "Send fd %v", proxyConnFD)
	// Send connection FD to child via socket
	_, _, err = clntConn.WriteMsgUnix(nil, oob, nil)
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error send conn fd: %v", err)
		return err
	}
	return nil
}

func (nps *NetProxySrv) Shutdown() {
	db.DPrintf(db.NETPROXYSRV, "Shutdown")
	os.Remove(sp.SIGMA_NETPROXY_SOCKET)
}
