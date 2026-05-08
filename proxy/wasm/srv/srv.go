package srv

import (
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"sigmaos/api/fs"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/proc"
	wasmproto "sigmaos/proxy/wasm/proto"
	wasmrt "sigmaos/proxy/wasm/rpc/wasmer"
	rpcsrv "sigmaos/rpc/srv"
	"sigmaos/rpc/transport"
	chunkclnt "sigmaos/sched/msched/proc/chunk/clnt"
	chunksrv "sigmaos/sched/msched/proc/chunk/srv"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/io/demux"
)

type WASMSrv struct {
	mu       sync.Mutex
	pe       *proc.ProcEnv
	sc       *sigmaclnt.SigmaClnt
	ckc      *chunkclnt.ChunkClnt
	kernelId string
}

func newWASMSrv(kernelId string) (*WASMSrv, error) {
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DPrintf(db.WASMD_ERR, "newWASMSrv NewSigmaClnt err: %v", err)
		return nil, err
	}
	ws := &WASMSrv{
		pe:       pe,
		sc:       sc,
		ckc:      chunkclnt.NewChunkClnt(sc.FsLib, false),
		kernelId: kernelId,
	}
	db.DPrintf(db.WASMD, "newWASMSrv ProcEnv:%v", pe)
	return ws, nil
}

func (ws *WASMSrv) runServer() error {
	dir := "/tmp/wasmd"
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}
	os.Remove(sp.WASMD_SOCKET)
	ln, err := net.Listen("unix", sp.WASMD_SOCKET)
	if err != nil {
		return err
	}
	if err := os.Chmod(sp.WASMD_SOCKET, 0777); err != nil {
		db.DFatalf("Err chmod wasmd socket: %v", err)
	}
	db.DPrintf(db.WASMD, "runServer: wasmd listening on %v", sp.WASMD_SOCKET)
	if _, err := io.WriteString(os.Stdout, "r"); err != nil {
		db.DFatalf("Err runServer write ready: %v", err)
		return err
	}
	go func() {
		buf := make([]byte, 1)
		if _, err := io.ReadFull(os.Stdin, buf); err != nil {
			db.DPrintf(db.WASMD_ERR, "read pipe err %v", err)
		}
		db.DPrintf(db.WASMD, "exiting")
		os.Remove(sp.WASMD_SOCKET)
		os.Exit(0)
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		newWASMSrvConn(conn, ws)
	}
}

// WASMSrvConn handles one long-lived client connection using the demux/rpc framework.
type WASMSrvConn struct {
	conn net.Conn
	ctx  fs.CtxI
	dmx  *demux.DemuxSrv
	rpcs *rpcsrv.RPCSrv
}

func newWASMSrvConn(conn net.Conn, ws *WASMSrv) *WASMSrvConn {
	api := &WASMSrvAPI{ws: ws}
	wsc := &WASMSrvConn{
		conn: conn,
		ctx:  ctx.NewCtxNull(),
	}
	iovm := demux.NewIoVecMap()
	wsc.rpcs = rpcsrv.NewRPCSrv(api, nil)
	wsc.dmx = demux.NewDemuxSrv(wsc, transport.NewTransport(conn, iovm))
	return wsc
}

func (wsc *WASMSrvConn) ServeRequest(c demux.CallI) (demux.CallI, *serr.Err) {
	req := c.(*transport.Call)
	rep, err := wsc.rpcs.WriteRead(wsc.ctx, req.Iov)
	if err != nil {
		db.DPrintf(db.WASMD_ERR, "ServeRequest WriteRead err %v", err)
	}
	return transport.NewCall(req.Seqno, rep), nil
}

func (wsc *WASMSrvConn) ReportError(err error) {
	db.DPrintf(db.WASMD_ERR, "WASMSrvConn ReportError err %v", err)
	go wsc.conn.Close()
}

// WASMSrvAPI implements the RPC methods served over each connection.
type WASMSrvAPI struct {
	ws *WASMSrv
}

func (api *WASMSrvAPI) RunWASMProc(ctx fs.CtxI, req wasmproto.RunWASMProcReq, rep *wasmproto.RunWASMProcRep) error {
	p := proc.NewProcFromProto(req.Proc)
	db.DPrintf(db.WASMD, "WASMSrvAPI.RunWASMProc %v", p.GetPid())
	status, msg, err := api.ws.runWASMProc(p)
	rep.Status = status
	rep.Msg = msg
	if err != nil && rep.Msg == "" {
		rep.Msg = err.Error()
	}
	return nil
}

func (ws *WASMSrv) runWASMProc(p *proc.Proc) (uint64, string, error) {
	pe := p.GetProcEnv()
	pe.UseDialProxy = false
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DPrintf(db.WASMD_ERR, "[%v] NewSigmaClnt err: %v", p.GetPid(), err)
		return 0, err.Error(), err
	}

	// Fetch the WASM binary via chunkd using the server's shared chunkclnt.
	st, _, err := ws.ckc.GetFileStat(ws.kernelId, p.GetVersionedProgram(), p.GetPid(), p.GetRealm(), p.GetSecrets()["s3"], p.GetSigmaPath(), p.GetNamedEndpoint())
	if err != nil {
		db.DPrintf(db.WASMD_ERR, "[%v] GetFileStat err: %v", p.GetPid(), err)
		return 0, "", err
	}
	_, err = ws.ckc.FetchBinary(ws.kernelId, p.GetVersionedProgram(), p.GetPid(), p.GetRealm(), p.GetSecrets()["s3"], st.Tsize(), p.GetSigmaPath(), p.GetNamedEndpoint())
	if err != nil {
		db.DPrintf(db.WASMD_ERR, "[%v] FetchBinary err: %v", p.GetPid(), err)
		return 0, err.Error(), err
	}
	localPath := filepath.Join(chunksrv.PathBinProc(), p.GetVersionedProgram())
	compiledModule, err := os.ReadFile(localPath)
	if err != nil {
		db.DPrintf(db.WASMD_ERR, "[%v] ReadFile err: %v", p.GetPid(), err)
		return 0, err.Error(), err
	}

	if pe.GetUseShmem() {
		sc.SetUseShmem(true)
	}
	rpcReps := NewRPCState()
	procAPI := NewWASMProcAPIImpl(ws, sc, p, rpcReps)
	wrt := wasmrt.NewWasmerRuntime(procAPI)
	wrt.SetRecvDelegated(procAPI.RecvDelegated)

	inputBytes := wasmrt.EncodeArgs(p.GetArgs())

	if err := sc.Started(); err != nil {
		db.DPrintf(db.WASMD_ERR, "[%v] Started err: %v", p.GetPid(), err)
		return 0, err.Error(), err
	}

	bufSz := wasmrt.DEFAULT_WASM_BUF_SZ
	if mb := p.GetWasmBufMB(); mb > 0 {
		bufSz = int(mb) * int(sp.MBYTE)
	}
	status, msg, runErr := wrt.RunModule(p.GetPid(), p.GetSpawnTime(), compiledModule, inputBytes, bufSz)
	db.DPrintf(db.WASMD, "[%v] Ran WASM proc, exit status:%v msg:%v err:%v", p.GetPid(), status, msg, runErr)

	exitStatus := proc.NewStatusInfo(proc.StatusOK, msg, msg)
	if runErr != nil || status != 0 {
		exitStatus = proc.NewStatus(proc.StatusErr)
	}
	sc.ClntExit(exitStatus)

	db.DPrintf(db.WASMD, "[%v] RunModule done status=%v msg=%v err=%v", p.GetPid(), status, msg, runErr)
	return uint64(status), msg, runErr
}

// RunWASMSrv is the entry point for the wasmd process.
func RunWASMSrv(kernelId string) error {
	ws, err := newWASMSrv(kernelId)
	if err != nil {
		db.DPrintf(db.WASMD_ERR, "newWASMSrv err %v", err)
		return err
	}
	return ws.runServer()
}

// WASMSrvCmd is a handle to a running wasmd subprocess.
type WASMSrvCmd struct {
	p   *proc.Proc
	cmd *exec.Cmd
	out io.WriteCloser
}

func (wsc *WASMSrvCmd) GetProc() *proc.Proc {
	return wsc.p
}

func (wsc *WASMSrvCmd) Shutdown() error {
	_, err := io.WriteString(wsc.out, "e")
	return err
}

func (wsc *WASMSrvCmd) Wait() error {
	if err := wsc.Shutdown(); err != nil {
		return err
	}
	return wsc.cmd.Wait()
}

// ExecWASMSrv starts the wasmd subprocess and waits for it to signal ready.
func ExecWASMSrv(p *proc.Proc, kernelId string, innerIP sp.Tip, outerIP sp.Tip, procdPid sp.Tpid) (*WASMSrvCmd, error) {
	p.FinalizeEnv(innerIP, outerIP, procdPid)
	db.DPrintf(db.WASMD, "ExecWASMSrv: %v", p)

	cmd := exec.Command("wasmd", kernelId)
	cmd.Env = p.GetEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		db.DPrintf(db.WASMD_ERR, "ExecWASMSrv read pipe err %v", err)
		return nil, err
	}
	return &WASMSrvCmd{
		p:   p,
		cmd: cmd,
		out: stdin,
	}, nil
}
