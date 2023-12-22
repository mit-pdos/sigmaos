// Package sigmaclntsrv is an RPC-based server that proxies the
// [sigmaos] interface over a pipe; it reads requests on stdin and
// write responses to stdout.
package sigmaclntsrv

import (
	"bufio"
	"net"
	"os"
	"os/exec"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/fs"
	"sigmaos/rpcsrv"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const SOCKET = "/tmp/sigmaclntd.sock"

type rpcCh struct {
	dmx  *demux.DemuxSrv
	rpcs *rpcsrv.RPCSrv
	ctx  fs.CtxI
}

func (rpcch *rpcCh) ServeRequest(f []byte) ([]byte, *serr.Err) {
	b, err := rpcch.rpcs.WriteRead(rpcch.ctx, f)
	if err != nil {
		db.DPrintf(db.SIGMACLNTSRV, "serveRPC: writeRead err %v\n", err)
	}
	return b, err
}

func newSigmaClntConn(conn net.Conn) error {
	db.DPrintf(db.SIGMACLNTSRV, "newSigmaClntConn for %v\n", conn)
	scs, err := NewSigmaClntSrv()
	if err != nil {
		return err
	}
	rpcs := rpcsrv.NewRPCSrv(scs, nil)
	rpcch := &rpcCh{nil, rpcs, ctx.NewCtxNull()}
	rpcch.dmx = demux.NewDemuxSrv(bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN),
		bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN), rpcch)
	return nil
}

func runServer() error {
	socket, err := net.Listen("unix", SOCKET)
	if err != nil {
		return err
	}
	db.DPrintf(db.SIGMACLNTSRV, "runServer: listening on %v\n", SOCKET)
	for {
		conn, err := socket.Accept()
		if err != nil {
			return err
		}
		newSigmaClntConn(conn)
	}
}

// The sigmaclntd process enter here
func RunSigmaClntSrv(args []string) error {
	if err := runServer(); err != nil {
		db.DPrintf(db.SIGMACLNTSRV, "runServer err %v\n", err)
		return err
	}
	return nil
}

// Start the sigmaclntd process
func ExecSigmaClntSrv() (*exec.Cmd, error) {
	cmd := exec.Command("../bin/linux/sigmaclntd", []string{}...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
