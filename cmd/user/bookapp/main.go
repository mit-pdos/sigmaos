package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"

	// db "ulambda/debug"
	"ulambda/dbd"
	"ulambda/fs"
	"ulambda/fsclnt"
	"ulambda/fslibsrv"
	//"ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
)

//
// book web app, invoked by wwwd
//

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid <name> args...\n", os.Args[0])
		os.Exit(1)
	}
	m, err := MakeBookApp(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	s := m.Work()
	m.Exit(s)
}

type BookApp struct {
	*fslibsrv.FsLibSrv
	proc.ProcClnt
	pid   string
	input []string
	pipe  fs.FsObj
}

func MakeBookApp(args []string) (*BookApp, error) {
	log.Printf("MakeBookApp: %v\n", args)

	ip, err := fsclnt.LocalIP()
	if err != nil {
		return nil, errors.New("MakeBookApp: No IP")
	}
	n := "name/" + args[2]
	memfsd := memfsd.MakeFsd(ip + ":0")
	pipe, err := memfsd.MkPipe("pipe")
	if err != nil {
		log.Fatal("Create error: ", err)
	}

	fsl, err := fslibsrv.InitFs(n, memfsd)
	if err != nil {
		return nil, err
	}

	r := &BookApp{}
	r.FsLibSrv = fsl
	r.ProcClnt = procinit.MakeProcClnt(fsl.FsLib, procinit.GetProcLayersMap())
	r.pid = args[1]
	r.input = strings.Split(args[3], "/")
	r.pipe = pipe
	r.Started(r.pid)

	return r, nil
}

func (ba *BookApp) writeResponse(data []byte) string {
	_, err := ba.pipe.(fs.File).Write(nil, 0, data, np.NoV)
	if err != nil {
		return fmt.Sprintf("Pipe parse err %v\n", err)
	}
	ba.ExitFs("name/" + ba.pid)
	return "OK"
}

func (ba *BookApp) query(q string) ([]byte, error) {
	b, err := ba.ReadFile(dbd.DBD + "clone")
	if err != nil {
		return nil, fmt.Errorf("Clone err %v\n", err)
	}
	sid := string(b)
	err = ba.WriteFile(dbd.DBD+sid+"/query", []byte(q))
	if err != nil {
		return nil, fmt.Errorf("Query err %v\n", err)
	}

	b, err = ba.ReadFile(dbd.DBD + sid + "/data")
	if err != nil {
		return nil, fmt.Errorf("Query response err %v\n", err)
	}
	return b, nil
}

func (ba *BookApp) doView() string {
	b, err := ba.query("select * from book;")
	if err != nil {
		return fmt.Sprintf("Query err %v\n", err)
	}

	var books []dbd.Book
	err = json.Unmarshal(b, &books)
	if err != nil {
		return fmt.Sprintf("Marshall err %v\n", err)
	}

	t, err := template.New("test").Parse(`<h1>Books</h1><ul>{{range .}}<li><a href="http://localhost:8080/edit/{{.Title}}">{{.Title}}</a> by {{.Author}}</li> {{end}}</ul>`)
	if err != nil {
		return fmt.Sprintf("Template parse err %v\n", err)
	}

	var data bytes.Buffer
	err = t.Execute(&data, books)
	if err != nil {
		return fmt.Sprintf("Template err %v\n", err)
	}

	log.Printf("bookapp: html %v\n", string(data.Bytes()))
	return ba.writeResponse(data.Bytes())
}

func (ba *BookApp) doEdit(key string) string {
	q := fmt.Sprintf("select * from book where title=\"%v\";", key)
	b, err := ba.query(q)
	if err != nil {
		return fmt.Sprintf("Query err %v\n", err)
	}

	var books []dbd.Book
	err = json.Unmarshal(b, &books)
	if err != nil {
		return fmt.Sprintf("Marshall err %v\n", err)
	}

	t, err := template.New("edit").Parse(`<h1>Editing {{.Title}}</h1>
<form action="/save/{{.Title}}" method="POST">
<div><textarea name="title" rows="20" cols="80">{{printf "%s" .Title}}</textarea></div>
<div><input type="submit" value="Save"></div>
</form>`)
	var data bytes.Buffer
	err = t.Execute(&data, books[0])
	if err != nil {
		return fmt.Sprintf("Template err %v\n", err)
	}

	log.Printf("bookapp: html %v\n", string(data.Bytes()))
	return ba.writeResponse(data.Bytes())
}

func (ba *BookApp) doSave(key string, title string) string {
	q := fmt.Sprintf("update book SET title=\"%v\" where title=\"%v\";", title, key)
	_, err := ba.query(q)
	if err != nil {
		return fmt.Sprintf("Query err %v\n", err)
	}
	return fmt.Sprintf("Redirect %v", "/book/view/")
}

func (ba *BookApp) Work() string {
	log.Printf("work %v\n", ba.input)
	_, err := ba.pipe.Open(nil, np.OWRITE)
	if err != nil {
		return fmt.Sprintf("Open err %v\n", err)
	}
	defer ba.pipe.Close(nil, np.OWRITE)

	switch ba.input[0] {
	case "view":
		return ba.doView()
	case "edit":
		return ba.doEdit(ba.input[1])
	case "save":
		return ba.doSave(ba.input[1], os.Args[4])
	default:
		return "File not found"
	}
}

func (ba *BookApp) Exit(status string) {
	log.Printf("bookapp exit %v\n", status)
	ba.Exited(ba.pid, status)
}
