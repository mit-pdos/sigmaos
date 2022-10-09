package www

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/fslib"
	"sigmaos/fslibsrv"
	np "sigmaos/ninep"
	"sigmaos/pipe"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/rand"
)

// HTTP server paths
const (
	STATIC = "/static/"
	MATMUL = "/matmul/"
	BOOK   = "/book/"
	EXIT   = "/exit/"
)

//
// Web front end that spawns an app to handle a request.
// XXX limit process's name space to the app binary and pipe.
//

var validPath = regexp.MustCompile(`^/(static|book|exit|matmul)/([=.a-zA-Z0-9/]*)$`)

func RunWwwd(job, tree string) {
	www := MakeWwwd(job, tree)
	http.HandleFunc(STATIC, www.makeHandler(getStatic))
	http.HandleFunc(BOOK, www.makeHandler(doBook))
	http.HandleFunc(EXIT, www.makeHandler(doExit))
	http.HandleFunc(MATMUL, www.makeHandler(doMatMul))

	go func() {
		www.Serve()
	}()

	ip, err := fidclnt.LocalIP()
	if err != nil {
		db.DFatalf("Error LocalIP: %v", err)
	}

	l, err := net.Listen("tcp", ip+":0")
	if err != nil {
		db.DFatalf("Error Listen: %v", err)
	}

	// Write a file for clients to discover the server's address.
	p := JobHTTPAddrsPath(job)
	www.PutFileJson(p, 0777, []string{l.Addr().String()})

	log.Fatal(http.Serve(l, nil))
}

type Wwwd struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	*fslibsrv.MemFs
	localSrvpath  string
	globalSrvpath string
}

func MakeWwwd(job, tree string) *Wwwd {
	www := &Wwwd{}

	var err error
	www.MemFs, www.FsLib, www.ProcClnt, err = fslibsrv.MakeMemFs(MemFsPath(job), WWWD)
	if err != nil {
		db.DFatalf("%v: MakeSrvFsLib %v %v\n", proc.GetProgram(), JobDir(job), err)
	}

	//	www.FsLib = fslib.MakeFsLibBase("www") // don't mount Named()
	// In order to automount children, we need to at least mount /pids.
	if err := procclnt.MountPids(www.FsLib, fslib.Named()); err != nil {
		db.DFatalf("wwwd err mount pids %v", err)
	}

	log.Printf("%v: pid %v procdir %v\n", proc.GetProgram(), proc.GetPid(), proc.GetProcDir())
	if _, err := www.PutFile(path.Join(np.TMP, "hello.html"), 0777, np.OWRITE, []byte("<html><h1>hello<h1><div>HELLO!</div></html>\n")); err != nil {
		db.DFatalf("wwwd MakeFile %v", err)
	}

	www.localSrvpath = path.Join(proc.PROCDIR, WWWD)
	www.globalSrvpath = path.Join(proc.GetProcDir(), WWWD)

	err = www.Symlink([]byte(MemFsPath(job)), www.localSrvpath, 0777)
	if err != nil {
		db.DFatalf("Error symlink memfs wwwd: %v", err)
	}
	return www
}

func (www *Wwwd) makeHandler(fn func(*Wwwd, http.ResponseWriter, *http.Request, string) (*proc.Status, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		}
	}
}

func (www *Wwwd) makePipe() string {
	// Make the pipe in the server.
	pipeName := rand.String(16)
	pipePath := path.Join(www.localSrvpath, pipeName)
	if err := www.MakePipe(pipePath, 0777); err != nil {
		db.DFatalf("%v: Error MakePipe %v", proc.GetProgram(), err)
	}
	return pipeName
}

func (www *Wwwd) removePipe(pipeName string) {
	pipePath := path.Join(www.localSrvpath, pipeName)
	if err := www.Remove(pipePath); err != nil {
		db.DFatalf("%v: Error Remove pipe %v", proc.GetProgram(), err)
	}
}

func (www *Wwwd) rwResponse(w http.ResponseWriter, pipeName string) {
	pipePath := path.Join(www.globalSrvpath, pipeName)
	db.DPrintf("WWW", "rwResponse: %v\n", pipePath)
	// Read from the pipe.
	fd, err := www.Open(pipePath, np.OREAD)
	if err != nil {
		log.Printf("wwwd: open %v failed %v", pipePath, err)
		return
	}
	defer www.Close(fd)
	for {
		b, err := www.Read(fd, pipe.PIPESZ)
		if err != nil || len(b) == 0 {
			break
		}
		//		log.Printf("wwwd: write %v\n", string(b))
		_, err = w.Write(b)
		if err != nil {
			break
		}
	}
	www.removePipe(pipeName)
}

func (www *Wwwd) spawnApp(app string, w http.ResponseWriter, r *http.Request, args []string) (*proc.Status, error) {
	// Create a pipe for the child to write to.
	pipeName := www.makePipe()

	pid := proc.GenPid()
	a := proc.MakeProcPid(pid, app, args)
	// Set the shared link to point to the pipe
	a.SetShared(path.Join(www.globalSrvpath, pipeName))
	err := www.Spawn(a)
	if err != nil {
		return nil, err
	}
	err = www.WaitStart(pid)
	if err != nil {
		return nil, err
	}
	// Read from the pipe in another thread. This way, if the child crashes or
	// terminates normally, we'll catch it with WaitExit and remove the pipe so
	// we don't block forever.
	go func() {
		www.rwResponse(w, pipeName)
	}()
	status, err := www.WaitExit(pid)
	return status, err
}

func getStatic(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) (*proc.Status, error) {
	log.Printf("%v: getstatic: %v\n", proc.GetProgram(), args)
	file := path.Join(np.TMP, args)
	return www.spawnApp("user/fsreader", w, r, []string{file})
}

func doBook(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) (*proc.Status, error) {
	log.Printf("dobook: %v\n", args)
	// XXX maybe pass all form key/values to app
	//r.ParseForm()
	//for key, value := range r.Form {
	//	log.Printf("form: %v %v", key, value)
	//}
	// log.Printf("\n")
	title := r.FormValue("title")
	return www.spawnApp("user/bookapp", w, r, []string{args, title})
}

func doExit(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) (*proc.Status, error) {
	www.Done()
	os.Exit(0)
	return nil, nil
}

func doMatMul(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) (*proc.Status, error) {
	log.Printf("matmul: %v\n", args)
	return www.spawnApp("user/matmul", w, r, []string{args})
}

func StopServer(pclnt *procclnt.ProcClnt, pid proc.Tpid) error {
	ch := make(chan error)
	go func() {
		_, err := exec.Command("wget", "-qO-", "http://localhost:8080/exit/").Output()
		ch <- err
	}()

	_, err := pclnt.WaitExit(pid)
	if err != nil {
		return err
	}

	<-ch
	return nil
}
