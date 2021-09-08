package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
	"ulambda/realm"
)

var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+.html)$")

func main() {
	www := MakeWwwd()
	http.HandleFunc("/view/", www.makeHandler(getPage))
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

	err := www.MakeFile("name/hello.html", 0777, np.OWRITE, []byte("hello\n"))
	if err != nil {
		log.Fatalf("wwwd MakeFile %v", err)
	}

	procinit.SetProcLayers(map[string]bool{procinit.PROCBASE: true})
	www.ProcClnt = procinit.MakeProcClnt(www.FsLib, procinit.GetProcLayersMap())
	db.Name("wwwd")
	return www
}

func (www *Wwwd) makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}

		pid := proc.GenPid()
		a := &proc.Proc{pid, "bin/user/fsreader", "",
			[]string{"name/hello.html", pid},
			[]string{procinit.GetProcLayersString()},
			proc.T_DEF, proc.C_DEF,
		}
		err := www.Spawn(a)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("spawned %v\n", m)
		err = www.WaitStart(pid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fn := "name/" + pid + "/pipe"
		log.Printf("open %v\n", fn)
		fd, err := www.Open(fn, np.OREAD)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		b, err := www.Read(fd, memfs.PIPESZ)
		if err != nil || len(b) == 0 {
			err := www.Close(fd)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		fmt.Fprintf(w, "<h1>%s</h1><div>%s</div>", "Hello", b)
	}
}

func getPage(w http.ResponseWriter, r *http.Request, title string) {
	//p, err := loadPage(title)
	//if err != nil {
	//	http.Redirect(w, r, "/edit/"+title, http.StatusFound)
	//	return
	//}
	http.NotFound(w, r)
}
