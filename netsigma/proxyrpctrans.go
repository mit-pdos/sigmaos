package netsigma

import (
	"net"
	"os"

	"golang.org/x/sys/unix"

	"google.golang.org/protobuf/proto"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/rpcsrv"
	sp "sigmaos/sigmap"
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

	p := sp.NoPrincipal()
	defer db.DPrintf(db.NETPROXYSRV, "Close conn principal %v", p)

	b, err := frame.ReadFrame(conn)
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error Read PrincipalID frame: %v", err)
		return
	}
	if err := proto.Unmarshal(b, p); err != nil {
		db.DPrintf(db.ERROR, "Error Unmarshal PrincipalID: %v", err)
		return
	}
	db.DPrintf(db.NETPROXYSRV, "Handle connection from principal %v", p)

	for {
		// Read the RPC
		req, err := frame.ReadFrames(conn)
		if err != nil {
			db.DPrintf(db.NETPROXYSRV_ERR, "Error ReadFrame p %v: %v", p.GetID(), err)
			return
		}
		ctx := NewWrapperCtx(ctx.NewPrincipalOnlyCtx(p))
		// Handle the RPC
		rep, err := npt.rpcs.WriteRead(ctx, req)
		if err != nil {
			db.DPrintf(db.NETPROXYSRV_ERR, "Error WriteRead p %v: %v", p.GetID(), err)
			return
		}
		db.DPrintf(db.NETPROXYSRV, "[%p] Write n frames: %v", conn, len(rep))
		// Send back the RPC response
		if err := frame.WriteFrames(conn, rep); err != nil {
			db.DPrintf(db.NETPROXYSRV_ERR, "Error WriteFrames p %v: %v", p.GetID(), err)
			return
		}
		if file, ok := ctx.GetFile(); ok {
			// Send back the FD, if a connection was successfully opened
			if err := sendProxiedFD(p, conn, file); err != nil {
				db.DPrintf(db.NETPROXYSRV_ERR, "Error send FD p %v: %v", p.GetID(), err)
				return
			}
			file.Close()
		} else {
			db.DPrintf(db.NETPROXYSRV_ERR, "Skipping sending FD %v: operation unsuccessful", p.GetID())
		}
	}
}

// Send the FD corresponding to the socket of the established (proxied)
// connection to the client.
func sendProxiedFD(p *sp.Tprincipal, conn *net.UnixConn, proxiedFile *os.File) error {
	fd := int(proxiedFile.Fd())
	oob := unix.UnixRights(fd)
	db.DPrintf(db.NETPROXYSRV, "Send p %v fd %v", p.GetID(), fd)
	// Send connection FD to child via socket
	_, _, err := conn.WriteMsgUnix(nil, oob, nil)
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error send conn fd (%v): %v", fd, err)
		return err
	}
	return nil
}
