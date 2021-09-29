package main

import (
	"log"
	"net/http"
	"regexp"

	"ulambda/dbd"
	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
	"ulambda/realm"
)

var validPath = regexp.MustCompile("^/(static|edit|save|view)/([.a-zA-Z0-9]+)$")

func main() {
	www := MakeWwwd()
	http.HandleFunc("/static/", www.makeHandler(getStatic))
	http.HandleFunc("/view/", www.makeHandler(getBook))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type Wwwd struct {
	*fslib.FsLib
	proc.ProcClnt
}

func MakeWwwd() *Wwwd {
	www := &Wwwd{}
	fsl := fslib.MakeFsLib("www")
	cfg := realm.GetRealmConfig(fsl, realm.TEST_RID)
	www.FsLib = fslib.MakeFsLibAddr("www", cfg.NamedAddr)

	err := www.MakeFile("name/hello.html", 0777, np.OWRITE, []byte("<html><h1>hello<h1><div>HELLO!</div></html>\n"))
	if err != nil {
		log.Fatalf("wwwd MakeFile %v", err)
	}

	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true})
	www.ProcClnt = procinit.MakeProcClnt(www.FsLib, procinit.GetProcLayersMap())
	db.Name("wwwd")
	return www
}

func (www *Wwwd) makeHandler(fn func(*Wwwd, http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(www, w, r, m[2])
	}
}

func (www *Wwwd) rwResponse(w http.ResponseWriter, pid string) {
	fn := "name/" + pid + "/pipe"
	log.Printf("open %v\n", fn)
	fd, err := www.Open(fn, np.OREAD)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for {
		b, err := www.Read(fd, memfs.PIPESZ)
		if err != nil || len(b) == 0 {
			// http.NotFound(w, r) on certain errors?
			break
		}
		_, err = w.Write(b)
		if err != nil {
			break
		}
	}
	defer www.Close(fd)
}

func getStatic(www *Wwwd, w http.ResponseWriter, r *http.Request, file string) {
	log.Printf("getpage: %v\n", file)
	pid := proc.GenPid()
	a := &proc.Proc{pid, "bin/user/fsreader", "",
		[]string{"name/" + file, pid},
		[]string{procinit.GetProcLayersString()},
		proc.T_DEF, proc.C_DEF,
	}
	err := www.Spawn(a)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = www.WaitStart(pid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	www.rwResponse(w, pid)
	status, err := www.WaitExit(pid)
	log.Printf("pid %v finished %v %v\n", pid, err, status)
}

func getBook(www *Wwwd, w http.ResponseWriter, r *http.Request, args string) {
	log.Printf("getbook %v\n", args)
	pid := proc.GenPid()
	a := &proc.Proc{pid, "bin/user/bookapp", "",
		[]string{dbd.DBD, pid},
		[]string{procinit.GetProcLayersString()},
		proc.T_DEF, proc.C_DEF,
	}
	err := www.Spawn(a)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = www.WaitStart(pid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	www.rwResponse(w, pid)
	status, err := www.WaitExit(pid)
	log.Printf("pid %v finished %v %v\n", pid, err, status)
}
