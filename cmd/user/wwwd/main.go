package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	//	"ulambda/dbd"
	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	//"ulambda/realm"
)

//
// Web front end that spawns an app to handle a request.
// XXX limit process's name space to the app binary and pipe.
//

var validPath = regexp.MustCompile(`^/(static|book|exit)/([=.a-zA-Z0-9/]*)$`)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <tree>\n", os.Args[0])
		os.Exit(1)
	}
	www := MakeWwwd(os.Args[1])
	http.HandleFunc("/static/", www.makeHandler(getStatic))
	http.HandleFunc("/book/", www.makeHandler(doBook))
	http.HandleFunc("/exit/", www.makeHandler(doExit))

	www.Started(proc.GetPid())

	log.Fatal(http.ListenAndServe(":8080", nil))
}

type Wwwd struct {
	*fslib.FsLib
	proc.ProcClnt
}

func MakeWwwd(tree string) *Wwwd {
	www := &Wwwd{}
	db.Name("wwwd")
	www.FsLib = fslib.MakeFsLibBase("www") // don't mount Named()

	log.Printf("%v: pid %v piddir %v\n", db.GetName(), proc.GetPid(), proc.GetPidDir())
	www.ProcClnt = procclnt.MakeProcClnt(www.FsLib)
	err := www.MakeFile("pids/hello.html", 0777, np.OWRITE, []byte("<html><h1>hello<h1><div>HELLO!</div></html>\n"))
	if err != nil {
		log.Fatalf("wwwd MakeFile %v", err)
	}
	return www
}

func (www *Wwwd) makeHandler(fn func(*Wwwd, http.ResponseWriter, *http.Request, string) (string, error)) http.HandlerFunc {
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
		if status == "File not found" {
			http.NotFound(w, r)
		} else if strings.HasPrefix(status, "Redirect") {
			t := strings.Split(status, " ")
			if len(t) > 1 {
				http.Redirect(w, r, t[1], http.StatusFound)
			}
		}
	}
}

func (www *Wwwd) rwResponse(w http.ResponseWriter, pid string) {
	fn := proc.PidDir(pid) + "/server/pipe"
	fd, err := www.Open(fn, np.OREAD)
	if err != nil {
		log.Printf("wwwd: open %v failed %v\n", fn, err)
		return
	}
	for {
		b, err := www.Read(fd, memfs.PIPESZ)
		if err != nil || len(b) == 0 {
			break
		}
		// log.Printf("wwwd: write %v\n", string(b))
		_, err = w.Write(b)
		if err != nil {
			break
		}
	}
	defer www.Close(fd)
}

func (www *Wwwd) spawnApp(app string, w http.ResponseWriter, r *http.Request, args []string) (string, error) {
	pid := proc.GenPid()
	a := proc.MakeProcPid(pid, app, append([]string{pid}, args...))
	a.PidDir = proc.GetPidDir()
	err := www.Spawn(a)
	if err != nil {
		return "", err
	}
	err = www.WaitStart(pid)
	if err != nil {
		return "", err
	}
	www.rwResponse(w, pid)
	return www.WaitExit(pid)
}

func getStatic(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) (string, error) {
	log.Printf("%v: getstatic: %v\n", db.GetName(), args)
	file := "name/" + proc.GetPidDir() + args
	return www.spawnApp("bin/user/fsreader", w, r, []string{file})
}

func doBook(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) (string, error) {
	log.Printf("dobook: %v\n", args)
	// XXX maybe pass all form key/values to app
	//r.ParseForm()
	//for key, value := range r.Form {
	//	log.Printf("form: %v %v", key, value)
	//}
	// log.Printf("\n")
	title := r.FormValue("title")
	return www.spawnApp("bin/user/bookapp", w, r, []string{args, title})
}

func doExit(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) (string, error) {
	www.Exited(proc.GetPid(), "OK")
	os.Exit(0)
	return "", nil
}
