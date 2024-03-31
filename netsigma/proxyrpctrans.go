package netsigma

import (
	"net"

	"golang.org/x/sys/unix"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/rpcsrv"
)

type NetProxyRPCTrans struct {
	nps    *NetProxySrv
	rpcs   *rpcsrv.RPCSrv
	socket net.Listener
}

func NewNetProxyRPCTrans(rpcs *rpcsrv.RPCSrv, socket net.Listener) *NetProxyRPCTrans {
	npt := &NetProxyRPCTrans{
		rpcs:   rpcs,
		socket: socket,
	}
	go npt.runTransport()
	return npt
}

func (npt *NetProxyRPCTrans) runTransport() error {
	for {
		conn, err := npt.socket.Accept()
		if err != nil {
			db.DFatalf("Error netproxysrv Accept: %v", err)
			return err
		}
		// Handle incoming connection
		go npt.handleNewConn(conn.(*net.UnixConn))
	}
}

func (npt *NetProxyRPCTrans) handleNewConn(conn *net.UnixConn) {
	defer conn.Close()

	for {
		// Read the RPC
		req, err := frame.ReadFrames(conn)
		if err != nil {
			db.DPrintf(db.ERROR, "Error ReadFrame: %v", err)
			return
		}
		ctx := NewWrapperCtx(ctx.NewCtxNull())
		// Handle the RPC
		rep, err := npt.rpcs.WriteRead(ctx, req)
		if err != nil {
			db.DPrintf(db.NETPROXYSRV_ERR, "Error WriteRead: %v", err)
			return
		}
		db.DPrintf(db.ALWAYS, "[%p] Write n frames: %v", conn, len(rep))
		// Send back the RPC response
		if err := frame.WriteFrames(conn, rep); err != nil {
			db.DPrintf(db.NETPROXYSRV_ERR, "Error WriteFrames: %v", err)
			return
		}
		if fd, ok := ctx.GetFD(); ok {
			// Send back the FD, if a connection was successfully opened
			if err := sendProxiedFD(conn, fd); err != nil {
				db.DPrintf(db.NETPROXYSRV_ERR, "Error send FD: %v", err)
				return
			}
		} else {
			db.DPrintf(db.NETPROXYSRV_ERR, "Skipping sending FD: operation unsuccessful")
		}
	}
}

// Send the FD corresponding to the socket of the established (proxied)
// connection to the client.
func sendProxiedFD(conn *net.UnixConn, proxiedFD int) error {
	oob := unix.UnixRights(proxiedFD)
	db.DPrintf(db.NETPROXYSRV, "Send fd %v", proxiedFD)
	// Send connection FD to child via socket
	_, _, err := conn.WriteMsgUnix(nil, oob, nil)
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error send conn fd: %v", err)
		return err
	}
	return nil
}
