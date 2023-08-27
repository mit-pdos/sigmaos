package www

import (
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"

	"net/http/pprof"

	"sigmaos/config"
	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/microbenchmarks"
	"sigmaos/pipe"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/rand"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

// HTTP server paths
const (
	TMP            = "name/tmp"
	STATIC         = "/static/"
	MATMUL         = "/matmul/"
	CONS_CPU_LOCAL = "/conscpulocal/"
	EXIT           = "/exit/"
	HELLO          = "/hello"
)

//
// Web front end that spawns an app to handle a request.
// XXX limit process's name space to the app binary and pipe.
//

var validPath = regexp.MustCompile(`^/(static|hotel|exit|matmul|user)/([=.a-zA-Z0-9/]*)$`)

func RunWwwd(job, tree string) {
	www := MakeWwwd(job, tree)
	http.HandleFunc(STATIC, www.makeHandler(getStatic))
	http.HandleFunc(HELLO, www.makeHandler(doHello))
	http.HandleFunc(EXIT, www.makeHandler(doExit))
	http.HandleFunc(MATMUL, www.makeHandler(doMatMul))
	http.HandleFunc(CONS_CPU_LOCAL, www.makeHandler(doConsumeCPULocal))
	http.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	http.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))

	ip, err := container.LocalIP()
	if err != nil {
		db.DFatalf("Error LocalIP: %v", err)
	}

	l, err := net.Listen("tcp", ip+":0")
	if err != nil {
		db.DFatalf("Error Listen: %v", err)
	}

	// Write a file for clients to discover the server's address.
	mnt := sp.MkMountService(sp.MkTaddrs([]string{l.Addr().String()}))
	if err = www.ssrv.SigmaClnt().MountService(JobHTTPAddrsPath(job), mnt, sp.NoLeaseId); err != nil {
		db.DFatalf("MountService %v", err)
	}

	go func() {
		www.ssrv.Serve()
	}()

	db.DFatalf("%v", http.Serve(l, nil))
}

type Wwwd struct {
	ssrv          *sigmasrv.SigmaSrv
	localSrvpath  string
	globalSrvpath string
}

func MakeWwwd(job, tree string) *Wwwd {
	www := &Wwwd{}

	scfg := config.GetSigmaConfig()
	var err error
	www.ssrv, err = sigmasrv.MakeSigmaSrvNoRPC(MemFsPath(job), scfg)
	if err != nil {
		db.DFatalf("%v: MakeSrvFsLib %v %v\n", proc.GetProgram(), JobDir(job), err)
	}

	//	www.FsLib = fslib.MakeFsLibBase("www") // don't mount Named()
	// In order to automount children, we need to at least mount /pids.
	if err := procclnt.MountPids(www.ssrv.SigmaClnt().FsLib, www.ssrv.SigmaClnt().NamedAddr()); err != nil {
		db.DFatalf("wwwd err mount pids %v", err)
	}

	db.DPrintf(db.ALWAYS, "%v: pid %v ", proc.GetProgram(), scfg.PID)
	if _, err := www.ssrv.SigmaClnt().PutFile(path.Join(TMP, "hello.html"), 0777, sp.OWRITE, []byte("<html><h1>hello<h1><div>HELLO!</div></html>\n")); err != nil && !serr.IsErrCode(err, serr.TErrExists) {
		db.DFatalf("wwwd MakeFile %v", err)
	}

	www.localSrvpath = path.Join(proc.PROCDIR, WWWD)
	www.globalSrvpath = path.Join(scfg.ProcDir, WWWD)

	err = www.ssrv.SigmaClnt().Symlink([]byte(MemFsPath(job)), www.localSrvpath, 0777)
	if err != nil {
		db.DFatalf("Error symlink memfs wwwd: %v", err)
	}
	return www
}

func (www *Wwwd) makeHandler(fn func(*Wwwd, http.ResponseWriter, *http.Request, string) (*proc.Status, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("path %v\n", r.URL.Path)
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		status, err := fn(www, w, r, m[2])
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		if status.IsStatusErr() && status.Msg() == "File not found" {
			http.NotFound(w, r)
		} else if status.IsStatusErr() && status.Msg() == "Redirect" {
			redirectUrl := status.Data().(string)
			http.Redirect(w, r, redirectUrl, http.StatusFound)
		} else if status.IsStatusErr() {
			http.Error(w, status.Msg(), http.StatusInternalServerError)
		}
	}
}

func (www *Wwwd) makePipe() string {
	// Make the pipe in the server.
	pipeName := rand.String(16)
	pipePath := path.Join(www.localSrvpath, pipeName)
	if err := www.ssrv.SigmaClnt().MakePipe(pipePath, 0777); err != nil {
		db.DFatalf("%v: Error MakePipe %v", proc.GetProgram(), err)
	}
	return pipeName
}

func (www *Wwwd) removePipe(pipeName string) {
	pipePath := path.Join(www.localSrvpath, pipeName)
	if err := www.ssrv.SigmaClnt().Remove(pipePath); err != nil {
		db.DFatalf("%v: Error Remove pipe %v", proc.GetProgram(), err)
	}
}

func (www *Wwwd) rwResponse(w http.ResponseWriter, pipeName string) {
	pipePath := path.Join(www.globalSrvpath, pipeName)
	db.DPrintf(db.WWW, "rwResponse: %v\n", pipePath)
	// Read from the pipe.
	fd, err := www.ssrv.SigmaClnt().Open(pipePath, sp.OREAD)
	if err != nil {
		db.DPrintf(db.WWW_ERR, "pipe open %v failed %v", pipePath, err)
		return
	}
	defer www.ssrv.SigmaClnt().Close(fd)
	for {
		b, err := www.ssrv.SigmaClnt().Read(fd, pipe.PIPESZ)
		if err != nil || len(b) == 0 {
			break
		}
		//		log.Printf("wwwd: write %v\n", string(b))
		_, err = w.Write(b)
		if err != nil {
			break
		}
	}
}

func (www *Wwwd) spawnApp(app string, w http.ResponseWriter, r *http.Request, pipe bool, args []string, env map[string]string, mcpu proc.Tmcpu) (*proc.Status, error) {
	var pipeName string
	pid := sp.GenPid()
	a := proc.MakeProcPid(pid, app, args)
	a.SetMcpu(mcpu)
	for k, v := range env {
		a.AppendEnv(k, v)
	}
	// Create a pipe for the child to write to.
	if pipe {
		pipeName = www.makePipe()
		// Set the shared link to point to the pipe
		a.SetShared(path.Join(www.globalSrvpath, pipeName))
	}
	db.DPrintf(db.WWW, "About to spawn %v", a)
	_, errs := www.ssrv.SigmaClnt().SpawnBurst([]*proc.Proc{a}, 1)
	if len(errs) != 0 {
		db.DFatalf("Error SpawnBurst %v", errs)
		return nil, errs[0]
	}
	db.DPrintf(db.WWW, "About to WaitStart %v", a)
	err := www.ssrv.SigmaClnt().WaitStart(pid)
	if err != nil {
		db.DFatalf("Error WaitStart %v", err)
		return nil, err
	}
	db.DPrintf(db.WWW, "Done WaitStart %v", a)
	if pipe {
		// Read from the pipe in another thread. This way, if the child crashes or
		// terminates normally, we'll catch it with WaitExit and remove the pipe so
		// we don't block forever.
		go func() {
			www.rwResponse(w, pipeName)
		}()
	}
	db.DPrintf(db.WWW, "About to WaitExit %v", a)
	status, err := www.ssrv.SigmaClnt().WaitExit(pid)
	db.DPrintf(db.WWW, "WaitExit done %v status %v err %v", pid, status, err)
	if pipe {
		www.removePipe(pipeName)
	}
	return status, err
}

func getStatic(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) (*proc.Status, error) {
	db.DPrintf(db.ALWAYS, "%v: getstatic: %v\n", proc.GetProgram(), args)
	file := path.Join(TMP, args)
	return www.spawnApp("fsreader", w, r, true, []string{file}, nil, 0)
}

func doHello(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) (*proc.Status, error) {
	_, err := w.Write([]byte("hello"))
	if err != nil {
		return nil, err
	}
	return proc.MakeStatus(proc.StatusOK), nil
}

func doExit(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) (*proc.Status, error) {
	www.ssrv.SrvExit(proc.MakeStatus(proc.StatusEvicted))
	os.Exit(0)
	return nil, nil
}

func doMatMul(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) (*proc.Status, error) {
	db.DPrintf(db.ALWAYS, "matmul: %v\n", args)
	return www.spawnApp("matmul", w, r, false, []string{args}, map[string]string{"GOMAXPROCS": "1"}, 1000)
}

// Consume some CPU with a simple CPU-bound task
func doConsumeCPULocal(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) (*proc.Status, error) {
	db.DPrintf(db.ALWAYS, "consumeCPULocal: %v\n", args)
	niter, err := strconv.Atoi(args)
	if err != nil {
		db.DFatalf("Can't convert niter %v", args)
	}
	microbenchmarks.ConsumeCPU(niter)
	return proc.MakeStatus(proc.StatusOK), nil
}
