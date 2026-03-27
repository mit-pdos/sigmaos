package srv

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/scontainer"
	sp "sigmaos/sigmap"
	"sigmaos/util/ivar"
	"sigmaos/util/linux/mem"
	"sigmaos/util/perf"
)

const (
	forkSockEnv                   = "SIGMA_FORK_SOCK"
	forkZygoteEnv                 = "SIGMA_FORK_ZYGOTE_KEY"
	defaultSockRel                = "/tmp/sigma_fork.sock"
	zygoteGracefulShutdownTimeout = 5 * time.Second
)

type ForkProc struct {
	hostPid     int
	zygotePid   sp.Tpid
	zygoteEntry *zygoteEntry
	didExit     ivar.IVar[error]
}

func (fp *ForkProc) Pid() int {
	return fp.hostPid
}

func (fp *ForkProc) GetPSS() (proc.Tmem, error) {
	pss, err := mem.GetAggregatePSS(fp.hostPid)
	if err != nil {
		return 0, err
	}

	// Add average PSS of zygote
	zygotePSS, err := mem.GetPSS(fp.zygoteEntry.zygCmd.Pid())
	db.DPrintf(db.ALWAYS, "ForkProc GetPSS: child host PID %d has PSS %d KB, zygote PID %d has PSS %d KB\n", fp.hostPid, pss, fp.zygoteEntry.zygCmd.Pid(), zygotePSS)

	if err != nil {
		return 0, err
	}
	zygotePSS = zygotePSS / proc.Tmem(fp.zygoteEntry.children)
	return pss + zygotePSS, nil
}

func (fp *ForkProc) Wait() error {
	return fp.didExit.Read()
}

func (fp *ForkProc) ZygotePid() sp.Tpid {
	return fp.zygotePid
}

type forkMsg struct {
	Type      string   `json:"type"`
	ZygoteKey string   `json:"zygote_key,omitempty"`
	ReqID     string   `json:"req_id,omitempty"`
	Env       []string `json:"env,omitempty"`
	Args      []string `json:"args,omitempty"`
}

func writeMsg(conn *net.UnixConn, msg any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = conn.Write(b)
	return err
}

func readMsg(conn *net.UnixConn, out any) error {
	buf := make([]byte, 64*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf[:n], out)
}

func randID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// --
type zygoteMap struct {
	byKey map[string]*zygoteEntry
	byPid map[sp.Tpid]*zygoteEntry
}

func newZygoteMap() zygoteMap {
	return zygoteMap{
		byKey: make(map[string]*zygoteEntry),
		byPid: make(map[sp.Tpid]*zygoteEntry),
	}
}

func (zm *zygoteMap) getByKey(key string) (*zygoteEntry, bool) {
	ze, ok := zm.byKey[key]
	return ze, ok
}

func (zm *zygoteMap) getByPid(pid sp.Tpid) (*zygoteEntry, bool) {
	ze, ok := zm.byPid[pid]
	return ze, ok
}

func (zm *zygoteMap) add(ze *zygoteEntry) {
	zm.byKey[ze.key] = ze
	zm.byPid[ze.zygProc.GetPid()] = ze
}

// Makes the zygote inaccessible for new forks by removing it from the byKey map.
func (zm *zygoteMap) makeInaccessible(ze *zygoteEntry) {
	if current, ok := zm.byKey[ze.key]; !ok || current != ze {
		return
	}
	delete(zm.byKey, ze.key)
}

// Removes the zygote from both maps unconditionally.
func (zm *zygoteMap) remove(ze *zygoteEntry) {
	delete(zm.byPid, ze.zygProc.GetPid())

	if current, ok := zm.byKey[ze.key]; !ok || current != ze {
		return
	}
	delete(zm.byKey, ze.key)
}

// zygoteState represents the lifecycle of a zygote process
type zygoteState int

const (
	stateStarting zygoteState = iota
	stateReady
	stateEvicting
	stateClosed
)

type forkMgr struct {
	ps      *ProcSrv
	mu      sync.RWMutex
	zygotes zygoteMap
}

type zygoteEntry struct {
	key       string
	keepAlive time.Duration

	// Lifecycle management
	ctx     context.Context
	cancel  context.CancelFunc
	stateMu sync.RWMutex
	state   zygoteState
	readyCh chan struct{} // closed when state transitions to stateReady

	// Connection management
	connMu   sync.Mutex
	listener *net.UnixListener
	zygConn  *net.UnixConn

	// Fork request tracking
	pendingMu sync.RWMutex
	pending   map[string]chan int

	// Child process tracking
	childMu  sync.Mutex
	children int
	lastIdle time.Time
	evictT   *time.Timer

	// Process management
	zygProc *proc.Proc
	zygCmd  *scontainer.UProcCmd
	exitErr error

	// Cleanup coordination
	wg sync.WaitGroup
}

func newForkMgr(ps *ProcSrv) *forkMgr {
	return &forkMgr{
		ps:      ps,
		zygotes: newZygoteMap(),
	}
}

func (fm *forkMgr) forkSockHostPath(pid sp.Tpid) string {
	return filepath.Join(scontainer.JailPath(pid), "tmp", filepath.Base(defaultSockRel))
}

// getState returns the current state safely
func (ze *zygoteEntry) getState() zygoteState {
	ze.stateMu.RLock()
	defer ze.stateMu.RUnlock()
	return ze.state
}

// setState transitions to a new state
func (ze *zygoteEntry) setState(newState zygoteState) {
	ze.stateMu.Lock()
	defer ze.stateMu.Unlock()
	ze.state = newState
	if newState == stateReady {
		close(ze.readyCh)
	}
}

// isUsable checks if the zygote can be used for forking
func (ze *zygoteEntry) isUsable() bool {
	state := ze.getState()
	return state == stateStarting || state == stateReady
}

func (fm *forkMgr) ensureZygote(uproc *proc.Proc) (*zygoteEntry, error) {
	fp := uproc.GetForkProc()
	if fp == nil {
		return nil, fmt.Errorf("ensureZygote called for non-fork proc")
	}
	key := fp.GetZygoteKey()

	// Fast path: check if we have a usable zygote
	fm.mu.RLock()
	if ze, ok := fm.zygotes.getByKey(key); ok && ze.isUsable() {
		fm.mu.RUnlock()
		return ze, nil
	}
	fm.mu.RUnlock()

	// Slow path: create new zygote under write lock
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Double-check after acquiring write lock
	if ze, ok := fm.zygotes.getByKey(key); ok && ze.isUsable() {
		return ze, nil
	}

	// Create new zygote entry
	ctx, cancel := context.WithCancel(context.Background())
	ze := &zygoteEntry{
		key:       key,
		keepAlive: time.Duration(fp.GetKeepAliveNs()),
		ctx:       ctx,
		cancel:    cancel,
		state:     stateStarting,
		readyCh:   make(chan struct{}),
		pending:   make(map[string]chan int),
	}

	// Build the zygote proc
	zyg := proc.NewProc(fp.GetZygoteProgram(), append([]string{}, fp.GetZygoteArgs()...))
	zyg.InheritParentProcEnv(uproc.GetProcEnv())
	zyg.SetRealm(uproc.GetRealm())
	for k, v := range fp.GetZygoteEnv() {
		zyg.AppendEnv(k, v)
	}
	zyg.GetProcEnv().UseSPProxy = false
	zyg.GetProcEnv().UseSPProxyProcClnt = false
	zyg.SetType(uproc.GetType())
	zyg.SetMcpu(uproc.GetMcpu())
	zyg.SetMem(uproc.GetMem())
	zyg.SetSpawnTime(uproc.GetSpawnTime())

	// Set up supervisor socket
	sockHost := fm.forkSockHostPath(zyg.GetPid())
	if err := os.MkdirAll(filepath.Dir(sockHost), 0777); err != nil {
		cancel()
		return nil, fmt.Errorf("mkdir fork sock dir: %w", err)
	}
	_ = os.Remove(sockHost)

	addr, err := net.ResolveUnixAddr("unixpacket", sockHost)
	if err != nil {
		cancel()
		return nil, err
	}

	listener, err := net.ListenUnix("unixpacket", addr)
	if err != nil {
		cancel()
		return nil, err
	}

	ze.listener = listener
	ze.zygProc = zyg

	zyg.AppendEnv(forkSockEnv, defaultSockRel)
	zyg.AppendEnv(forkZygoteEnv, key)

	// Assign to realm
	_, isPythonProc := zyg.LookupEnv("SIGMA_PYTHON_VERSION")
	var stringProg string
	if isPythonProc {
		stringProg = zyg.GetProgram()
	} else {
		stringProg = zyg.GetVersionedProgram()
	}

	if err := fm.ps.assignToRealm(zyg.GetRealm(), zyg.GetPid(), stringProg,
		zyg.GetSigmaPath(), zyg.GetSecrets()["s3"], zyg.GetNamedEndpoint()); err != nil {
		cancel()
		_ = listener.Close()
		return nil, err
	}

	zyg.FinalizeEnv(fm.ps.pe.GetInnerContainerIP(),
		fm.ps.pe.GetOuterContainerIP(), fm.ps.pe.GetPID())

	// Start the zygote container
	cmd, err := scontainer.StartSigmaContainer(zyg, fm.ps.dialproxy, fm.ps.sc, fm.ps.pyenvClnt)
	if err != nil {
		cancel()
		_ = listener.Close()
		return nil, err
	}
	ze.zygCmd = cmd

	// Register in map before starting goroutines
	fm.zygotes.add(ze)

	// Start background goroutines
	ze.wg.Add(2)
	go fm.acceptLoop(ze)
	go fm.monitorZygote(ze)

	return ze, nil
}

func (fm *forkMgr) acceptLoop(ze *zygoteEntry) {
	defer ze.wg.Done()

	for {
		select {
		case <-ze.ctx.Done():
			return
		default:
		}

		// Set deadline to allow periodic context checks
		ze.connMu.Lock()
		listener := ze.listener
		ze.connMu.Unlock()

		if listener == nil {
			return
		}

		_ = listener.SetDeadline(time.Now().Add(1 * time.Second))
		conn, err := listener.AcceptUnix()

		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			select {
			case <-ze.ctx.Done():
				return
			default:
				db.DPrintf(db.PROCD_ERR, "forkmgr accept error: %v", err)
				continue
			}
		}

		ze.wg.Add(1)
		go fm.handleConn(ze, conn)
	}
}

func (fm *forkMgr) handleConn(ze *zygoteEntry, conn *net.UnixConn) {
	defer ze.wg.Done()

	var m forkMsg
	if err := readMsg(conn, &m); err != nil {
		return
	}

	switch m.Type {
	case "hello":
		fm.handleHello(ze, conn, &m)
	default:
		_ = writeMsg(conn, forkMsg{Type: "err"})
		conn.Close()
	}
}

func (fm *forkMgr) handleHello(ze *zygoteEntry, conn *net.UnixConn, m *forkMsg) {
	if m.ZygoteKey != ze.key {
		_ = writeMsg(conn, forkMsg{Type: "err"})
		return
	}

	ze.connMu.Lock()
	defer ze.connMu.Unlock()

	if ze.zygConn != nil {
		// Already have a connection
		conn.Close()
		return
	}

	// Set SO_PASSCRED on the accepted connection to get host PID of future messages
	rawConn, err := conn.File()
	if err != nil {
		ze.connMu.Unlock()
		conn.Close()
		return
	}
	defer rawConn.Close()

	err = unix.SetsockoptInt(int(rawConn.Fd()), unix.SOL_SOCKET, unix.SO_PASSCRED, 1)
	if err != nil {
		db.DPrintf(db.PROCD_ERR, "Failed to set SO_PASSCRED: %v", err)
		ze.connMu.Unlock()
		conn.Close()
		return
	}

	ze.zygConn = conn
	ze.setState(stateReady)
	perf.LogSpawnLatency("Zygote ready", ze.zygProc.GetPid(), ze.zygProc.GetSpawnTime(), perf.TIME_NOT_SET)

	ze.childMu.Lock()
	ze.lastIdle = time.Now()
	ze.childMu.Unlock()

	_ = writeMsg(conn, forkMsg{Type: "ok"})

	// From here on out, we expect all incoming messages to be of type "child".
	ze.wg.Add(1)
	go fm.zygoteStreamReader(ze, conn)

	// Additionally, we may close the listener, as we've sucesfully established a connection.
	go func() {
		listener := ze.listener
		if listener != nil {
			_ = listener.Close()
		}
	}()
}

func (fm *forkMgr) zygoteStreamReader(ze *zygoteEntry, conn *net.UnixConn) {
	defer ze.wg.Done()
	defer conn.Close()

	rawConn, _ := conn.File()
	defer rawConn.Close()
	sockFd := int(rawConn.Fd())

	buf := make([]byte, 16*1024)
	oob := make([]byte, unix.CmsgSpace(unix.SizeofUcred))

	for {
		// Use unix.Select to wait for data with a timeout.
		// This is more expensive than I would like, but for now, it is the best solution
		// that I could come up with, as otherwise, calling conn.Close() while we're still
		// trying to conn.ReadMsgUnix, would block indefinitely.
		rfds := &unix.FdSet{}
		rfds.Set(sockFd)

		_, err := unix.Select(sockFd+1, rfds, nil, nil, &unix.Timeval{Sec: 1})
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return
		}

		select {
		case <-ze.ctx.Done():
			return
		default:
		}

		if !rfds.IsSet(sockFd) {
			continue
		}

		// Data available -> read
		n, oobn, _, _, err := conn.ReadMsgUnix(buf, oob)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				db.DPrintf(db.PROCD_ERR, "Zygote stream read error: %v", err)
			}
			return
		}

		var hostPid int
		if oobn > 0 {
			msgs, err := unix.ParseSocketControlMessage(oob[:oobn])
			if err == nil {
				for _, msg := range msgs {
					if msg.Header.Level == unix.SOL_SOCKET &&
						msg.Header.Type == unix.SCM_CREDENTIALS {
						ucred, err := unix.ParseUnixCredentials(&msg)
						if err == nil {
							hostPid = int(ucred.Pid)
						}
					}
				}
			}
		}

		var m forkMsg
		if err := json.Unmarshal(buf[:n], &m); err != nil {
			continue
		}

		if m.Type == "child" {
			ze.pendingMu.Lock()
			ch := ze.pending[m.ReqID]
			if ch != nil {
				delete(ze.pending, m.ReqID)
			}
			ze.pendingMu.Unlock()

			if ch != nil {
				select {
				case ch <- hostPid:
				default:
				}
				close(ch)
			}
		}
	}
}

// Monitor the zygote and trigger cleanup on exit
func (fm *forkMgr) monitorZygote(ze *zygoteEntry) {
	err := ze.zygCmd.Wait()
	ze.exitErr = err

	db.DPrintf(db.PROCD, "Zygote %s (key: %s) exited (err: %v)\n", ze.zygProc.GetPid(), ze.key, err)

	ze.wg.Done()

	// CLEAN UP
	fm.mu.Lock()
	fm.zygotes.makeInaccessible(ze)
	ze.setState(stateClosed)
	ze.cancel()
	fm.mu.Unlock()

	// Close all connections
	ze.connMu.Lock()
	if ze.listener != nil {
		_ = ze.listener.Close()
		ze.listener = nil
	}
	if ze.zygConn != nil {
		_ = ze.zygConn.Close()
		ze.zygConn = nil
	}
	ze.connMu.Unlock()

	// Cancel all pending fork requests
	ze.pendingMu.Lock()
	for _, ch := range ze.pending {
		close(ch)
	}
	ze.pending = make(map[string]chan int)
	ze.pendingMu.Unlock()

	// Stop eviction timer
	ze.childMu.Lock()
	if ze.evictT != nil {
		ze.evictT.Stop()
		ze.evictT = nil
	}
	ze.childMu.Unlock()

	// Wait for all goroutines to finish, and all children to exit
	ze.wg.Wait()
	scontainer.CleanupUProc(ze.zygCmd, fm.ps.pyenvClnt)

	fm.mu.Lock()
	fm.zygotes.remove(ze)
	fm.mu.Unlock()
}

// Ensures a matching zygote is running and requests it to fork a child proc.
// Returns the host PID of the forked child, and a unique ID for the zygote.
func (fm *forkMgr) forkChild(uproc *proc.Proc) (*ForkProc, error) {
	start := time.Now()
	perf.LogSpawnLatency("forkMgr.forkChild start", uproc.GetPid(), uproc.GetSpawnTime(), perf.TIME_NOT_SET)

	fp := uproc.GetForkProc()
	if fp == nil {
		return nil, fmt.Errorf("forkChild called for non-fork proc")
	}

	ze, err := fm.ensureZygote(uproc)
	if err != nil {
		return nil, err
	}

	// Wait for zygote to be ready
	select {
	case <-ze.readyCh:
		// Ready to proceed
	case <-ze.ctx.Done():
		return nil, fmt.Errorf("zygote exited before ready: %v", ze.exitErr)
	case <-time.After(1 * time.Minute):
		return nil, fmt.Errorf("zygote setup timed out")
	}

	// Verify still usable after waiting
	if !ze.isUsable() {
		return nil, fmt.Errorf("zygote became unusable")
	}

	reqID, err := randID()
	if err != nil {
		return nil, err
	}

	respCh := make(chan int, 1)

	// Register pending request
	ze.pendingMu.Lock()
	ze.pending[reqID] = respCh
	ze.pendingMu.Unlock()

	defer func() {
		ze.pendingMu.Lock()
		delete(ze.pending, reqID)
		ze.pendingMu.Unlock()
	}()

	// Send fork request
	ze.connMu.Lock()
	conn := ze.zygConn
	ze.connMu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("zygote connection missing")
	}

	uproc.AppendEnv(proc.SIGMADEBUGPID, uproc.GetPid().String())
	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	b, err := time.Now().MarshalText()
	if err != nil {
		db.DFatalf("Error marshal timestamp pb: %v", err)
	}
	uproc.AppendEnv("SIGMA_EXEC_TIME_PB", string(b))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))

	perf.LogSpawnLatency("forkMgr.forkChild send fork request", uproc.GetPid(), uproc.GetSpawnTime(), start)
	err = writeMsg(conn, forkMsg{
		Type:  "fork",
		ReqID: reqID,
		Env:   uproc.GetEnv(),
		Args:  fp.GetChildArgs(),
	})
	if err != nil {
		return nil, err
	}

	// Wait for response
	select {
	case hostPid, ok := <-respCh:
		if !ok {
			return nil, fmt.Errorf("zygote closed while waiting for fork")
		}
		ze.childMu.Lock()
		ze.wg.Add(1)
		ze.children++
		ze.childMu.Unlock()

		zygotePid := ze.zygProc.GetPid()
		forkProc := ForkProc{
			hostPid:     hostPid,
			zygotePid:   zygotePid,
			zygoteEntry: ze,
			didExit:     ivar.NewIVar[error](),
		}

		go func() {
			err := waitForHostPIDExit(hostPid)
			fm.childDone(zygotePid)
			forkProc.didExit.Fill(err)
		}()

		perf.LogSpawnLatency("forkMgr.forkChild done", uproc.GetPid(), uproc.GetSpawnTime(), start)

		return &forkProc, nil
	case <-ze.ctx.Done():
		return nil, fmt.Errorf("zygote exited while forking: %v", ze.exitErr)
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout waiting for forked child")
	}
}

func (fm *forkMgr) childDone(zygotePid sp.Tpid) {
	fm.mu.RLock()
	ze, _ := fm.zygotes.getByPid(zygotePid)
	fm.mu.RUnlock()

	if ze == nil {
		return
	}

	ze.childMu.Lock()
	defer ze.childMu.Unlock()

	if ze.children > 0 {
		ze.children--
		ze.wg.Done()
	}
	ze.lastIdle = time.Now()

	// If no children remaining, schedule eviction if keepAlive configured
	if ze.children == 0 {
		if ze.keepAlive <= 0 {
			// Evict immediately
			go fm.tryEvict(ze)
		} else {
			// Schedule eviction
			if ze.evictT != nil {
				ze.evictT.Stop()
			}
			ze.evictT = time.AfterFunc(ze.keepAlive, func() {
				fm.tryEvict(ze)
			})
		}
	}
}

func (fm *forkMgr) tryEvict(ze *zygoteEntry) {
	fm.mu.Lock()

	zeState := ze.getState()
	if zeState == stateEvicting || zeState == stateClosed {
		fm.mu.Unlock()
		return
	}

	ze.childMu.Lock()

	// Re-check the eviction condition
	shouldEvict := ze.children == 0
	if ze.keepAlive > 0 {
		shouldEvict = shouldEvict && time.Since(ze.lastIdle) >= ze.keepAlive
	}

	if !shouldEvict {
		ze.childMu.Unlock()
		fm.mu.Unlock()
		return
	}
	ze.childMu.Unlock()

	// Point of no return
	ze.setState(stateEvicting)
	fm.zygotes.makeInaccessible(ze)
	ze.cancel()
	fm.mu.Unlock()

	// Close connections to trigger shutdown.
	// We do this in a separate goroutine to avoid blocking on the .Close call.
	go func() {
		ze.connMu.Lock()
		defer ze.connMu.Unlock()

		if ze.listener != nil {
			_ = ze.listener.Close()
			ze.listener = nil
		}
		if ze.zygConn != nil {
			_ = ze.zygConn.Close()
			ze.zygConn = nil
		}
	}()

	// Wait for graceful shutdown with timeout
	done := make(chan struct{})
	go func() {
		ze.zygCmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(zygoteGracefulShutdownTimeout):
		if ze.zygCmd != nil {
			_ = ze.zygCmd.Kill()
		}
	}
}

func waitForHostPIDExit(hostPid int) error {
	fd, err := unix.PidfdOpen(hostPid, 0)
	if err != nil {
		return fmt.Errorf("pidfd_open(%d): %w", hostPid, err)
	}
	defer unix.Close(fd)

	if err := unix.PidfdSendSignal(fd, 0, nil, 0); err != nil {
		return nil // Already exited
	}

	for {
		_, err = unix.Poll([]unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}, -1)
		if err == nil || !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("pidfd poll(%d): %w", hostPid, err)
	}
	return nil
}
