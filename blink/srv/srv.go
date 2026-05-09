package srv

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"sigmaos/api/fs"
	"sigmaos/blink"
	blinkproto "sigmaos/blink/proto"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/proc"
	rpcsrv "sigmaos/rpc/srv"
	"sigmaos/rpc/transport"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/util/io/demux"
)

type BlinkSrv struct {
	kernelId string
}

func newBlinkSrv(kernelId string) *BlinkSrv {
	return &BlinkSrv{kernelId: kernelId}
}

func (bs *BlinkSrv) runServer() error {
	addr := fmt.Sprintf(":%d", blink.BLINK_PORT)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	db.DPrintf(db.BLINKD, "BlinkSrv listening on %v", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		newBlinkSrvConn(conn, bs)
	}
}

type BlinkSrvConn struct {
	conn net.Conn
	ctx  fs.CtxI
	dmx  *demux.DemuxSrv
	rpcs *rpcsrv.RPCSrv
}

func newBlinkSrvConn(conn net.Conn, bs *BlinkSrv) *BlinkSrvConn {
	api := &BlinkSrvAPI{bs: bs}
	bsc := &BlinkSrvConn{
		conn: conn,
		ctx:  ctx.NewCtxNull(),
	}
	iovm := demux.NewIoVecMap()
	bsc.rpcs = rpcsrv.NewRPCSrv(api, nil)
	bsc.dmx = demux.NewDemuxSrv(bsc, transport.NewTransport(conn, iovm))
	return bsc
}

func (bsc *BlinkSrvConn) ServeRequest(c demux.CallI) (demux.CallI, *serr.Err) {
	req := c.(*transport.Call)
	rep, err := bsc.rpcs.WriteRead(bsc.ctx, req.Iov)
	if err != nil {
		db.DPrintf(db.BLINKD, "BlinkSrvConn ServeRequest err %v", err)
	}
	return transport.NewCall(req.Seqno, rep), nil
}

func (bsc *BlinkSrvConn) ReportError(err error) {
	db.DPrintf(db.BLINKD, "BlinkSrvConn ReportError err %v", err)
	go bsc.conn.Close()
}

type BlinkSrvAPI struct {
	bs *BlinkSrv
}

func (api *BlinkSrvAPI) RunBlinkProc(ctx fs.CtxI, req blinkproto.RunBlinkProcReq, rep *blinkproto.RunBlinkProcRep) error {
	p := proc.NewProcFromProto(req.Proc)
	db.DPrintf(db.BLINKD, "BlinkSrvAPI.RunBlinkProc %v kid %v procsrvPID %v", p, req.Kid, req.ProcsrvPid)

	// Mount the procsrv's spproxyd socket dir into the chroot.
	chrootSpproxyd := blink.JUNCTION_CHROOT + "/tmp/spproxyd"
	//hostSpproxyd := blink.PROCD_SPPROXYD_BASE + "/spproxyd-" + req.ProcsrvPid
	if err := exec.Command("sudo", "mkdir", "-p", chrootSpproxyd).Run(); err != nil {
		db.DPrintf(db.ERROR, "ERR RunBlinkProc mkdir chroot spproxyd: %v", err)
		rep.Err = sp.NewRerrorErr(fmt.Errorf("mkdir chroot spproxyd: %w", err))
		return nil
	}
	//	if err := exec.Command("sudo", "mount", "--bind", hostSpproxyd, chrootSpproxyd).Run(); err != nil {
	//		db.DPrintf(db.ERROR, "ERR RunBlinkProc mount spproxyd: %v", err)
	//		return fmt.Errorf("mount spproxyd: %w", err)
	//	}
	//	if err := exec.Command("sudo", "chmod", "a+rwx", chrootSpproxyd).Run(); err != nil {
	//		db.DPrintf(db.ERROR, "ERR RunBlinkProc chmod spproxyd: %v", err)
	//		return fmt.Errorf("chmod spproxyd: %w", err)
	//	}
	//	defer exec.Command("sudo", "umount", chrootSpproxyd).Run()

	program := strings.TrimSuffix(p.GetProgram(), ".py")
	functionName := "python_" + program
	snapshotPrefix := "/tmp/python_" + program

	// Build env map from proc env vars to pass via functionArg.
	envMap := make(map[string]string)
	for _, envVar := range p.GetEnv() {
		key, val, found := strings.Cut(envVar, "=")
		if found {
			envMap[key] = val
		}
	}

	// Args: [ImgBucket, ImgKey, ModelBucket, ModelKey, Kid, AsyncFetch]
	functionArg := map[string]interface{}{
		"is_warmup":        "false",
		"img_bucket":       p.Args[0],
		"img_key":          p.Args[1],
		"model_bucket":     p.Args[2],
		"model_key":        p.Args[3],
		"kid":              p.Args[4],
		"async_fetch":      p.Args[5],
		"spproxy_tcp_host": blink.DTAP0_ADDR,
		"spproxy_tcp_port": fmt.Sprintf("%d", req.SpproxyTcpPort),
		"env":              envMap,
	}
	functionArgJSON, err := json.Marshal(functionArg)
	if err != nil {
		db.DPrintf(db.ERROR, "ERR RunBlinkProc marshal function_arg: %v", err)
		rep.Err = sp.NewRerrorErr(fmt.Errorf("marshal function_arg: %w", err))
		return nil
	}

	args := []string{"-E", blink.JUNCTION_RUN, blink.JUNCTION_CONFIG,
		"--function_arg", string(functionArgJSON),
		"--function_name", functionName,
	}
	straceProcs := proc.GetLabels(p.ProcEnvProto.GetStrace())
	if straceProcs[program] {
		args = append(args, "--strace")
		db.DPrintf(db.BLINKD, "Stracing %v", p.GetPid())
	}
	args = append(args,
		"--chroot="+blink.JUNCTION_CHROOT,
		"--cache_linux_fs",
		"--jif",
		"-rk",
		"--",
		snapshotPrefix+".jm",
		snapshotPrefix+blink.SNAPSHOT_JIF_SUFFIX,
	)

	logFile, err := os.OpenFile("/tmp/blinkd-restore.out", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		db.DPrintf(db.ERROR, "ERR RunBlinkProc open restore log: %v", err)
		rep.Err = sp.NewRerrorErr(fmt.Errorf("open restore log: %w", err))
		return nil
	}
	defer logFile.Close()

	procArgs := []string{
		"img_bucket=" + p.Args[0],
		"img_key=" + p.Args[1],
		"model_bucket=" + p.Args[2],
		"model_key=" + p.Args[3],
		"kid=" + p.Args[4],
		"async_fetch=" + p.Args[5],
		"spproxy_tcp_host=" + blink.DTAP0_ADDR,
		"spproxy_tcp_port=" + fmt.Sprintf("%d", req.SpproxyTcpPort),
	}
	allLines := append(p.GetEnv(), procArgs...)
	if err := os.WriteFile("/tmp/proc-env.txt", []byte(strings.Join(allLines, "\n")+"\n"), 0644); err != nil {
		db.DPrintf(db.ERROR, "ERR RunBlinkProc write proc env: %v", err)
		rep.Err = sp.NewRerrorErr(fmt.Errorf("write proc env: %w", err))
		return nil
	}
	cmd := exec.Command("sudo", args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	db.DPrintf(db.BLINKD, "BlinkSrvAPI.RunBlinkProc exec: %v", strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		db.DPrintf(db.ERROR, "ERR junction_run: %v", err)
		rep.Err = sp.NewRerrorErr(fmt.Errorf("junction_run: %w", err))
		return nil
	}
	rep.Err = sp.NewRerror()
	return nil
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func setupCaladan() error {
	if err := runCmd("sudo", "sh", "-c", blink.CHROOT_MOUNT_SCRIPT+" -u || true"); err != nil {
		return fmt.Errorf("chroot_mount unmount: %w", err)
	}
	if err := runCmd("sudo", blink.CHROOT_MOUNT_SCRIPT); err != nil {
		return fmt.Errorf("chroot_mount: %w", err)
	}
	if err := runCmd("sudo", "sh", "-c", "(pkill iokerneld && sleep 1) || true"); err != nil {
		return fmt.Errorf("pkill iokerneld: %w", err)
	}
	if err := runCmd("sudo", blink.CALADAN_SETUP_SCRIPT, "nouintr"); err != nil {
		return fmt.Errorf("setup_machine.sh: %w", err)
	}
	logFile, err := os.Create(blink.BLINK_RESULTS + "/generate_images_iokernel.log")
	if err != nil {
		return fmt.Errorf("create iokerneld log: %w", err)
	}
	if err := runCmd("sudo", "sh", "-c", "echo 3 | tee /proc/sys/vm/drop_caches"); err != nil {
		return fmt.Errorf("drop caches: %w", err)
	}
	cmd := exec.Command("sudo", blink.IOKERNELD_BIN, "ias", "noht", "nobw", "no_hw_qdel", "numanode", "-1", "--", "--allow", "00:00.0", "--vdev=net_tap0", "1,4-28")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start iokerneld: %w", err)
	}
	db.DPrintf(db.BLINKD, "iokerneld started (pid %d), logging to %s", cmd.Process.Pid, blink.BLINK_RESULTS+"/generate_images_iokernel.log")
	// Wait for iokerneld to create dtap0 before assigning the address.
	for i := 0; ; i++ {
		if exec.Command("ip", "link", "show", "dtap0").Run() == nil {
			break
		}
		if i >= 5000 {
			return fmt.Errorf("dtap0 did not appear after iokerneld start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := runCmd("sudo", "ip", "addr", "add", blink.DTAP0_ADDR+"/16", "dev", "dtap0"); err != nil {
		return fmt.Errorf("ip addr add dtap0: %w", err)
	}
	if err := runCmd("sudo", "sysctl", "-w", "net.ipv4.ip_forward=1"); err != nil {
		return fmt.Errorf("enable ip_forward: %w", err)
	}
	db.DPrintf(db.BLINKD, "Caladan setup done")
	return nil
}

func RunBlinkSrv(kernelId string) error {
	if err := setupCaladan(); err != nil {
		return err
	}
	bs := newBlinkSrv(kernelId)
	return bs.runServer()
}
