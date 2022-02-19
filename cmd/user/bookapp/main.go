package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"path"
	"strings"

	// db "ulambda/debug"
	"ulambda/dbd"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

//
// book web app, invoked by wwwd
//

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <name> args...\n", os.Args[0])
		os.Exit(1)
	}
	m, err := RunBookApp(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	s := m.Work()
	m.Exit(s)
}

type BookApp struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	input  []string
	pipefd int
}

func RunBookApp(args []string) (*BookApp, error) {
	log.Printf("MakeBookApp: %v\n", args)
	ba := &BookApp{}
	ba.FsLib = fslib.MakeFsLib("bookapp")
	ba.ProcClnt = procclnt.MakeProcClnt(ba.FsLib)
	ba.input = strings.Split(args[2], "/")
	ba.Started(proc.GetPid())

	return ba, nil
}

func (ba *BookApp) writeResponse(data []byte) *proc.Status {
	_, err := ba.Write(ba.pipefd, data)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Pipe parse err %v\n", err), nil)
	}
	ba.Evict(proc.GetPid())
	return proc.MakeStatus(proc.StatusOK)
}

func (ba *BookApp) query(q string) ([]byte, error) {
	b, err := ba.ReadFile(dbd.DBD + "clone")
	if err != nil {
		return nil, fmt.Errorf("Clone err %v\n", err)
	}
	sid := string(b)
	_, err = ba.SetFile(dbd.DBD+sid+"/query", []byte(q), 0)
	if err != nil {
		return nil, fmt.Errorf("Query err %v\n", err)
	}

	b, err = ba.ReadFile(dbd.DBD + sid + "/data")
	if err != nil {
		return nil, fmt.Errorf("Query response err %v\n", err)
	}
	return b, nil
}

func (ba *BookApp) doView() *proc.Status {
	b, err := ba.query("select * from book;")
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Query err %v\n", err), nil)
	}

	var books []dbd.Book
	err = json.Unmarshal(b, &books)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Marshall err %v\n", err), nil)
	}

	t, err := template.New("test").Parse(`<h1>Books</h1><ul>{{range .}}<li><a href="http://localhost:8080/edit/{{.Title}}">{{.Title}}</a> by {{.Author}}</li> {{end}}</ul>`)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Template parse err %v\n", err), nil)
	}

	var data bytes.Buffer
	err = t.Execute(&data, books)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Template err %v\n", err), nil)
	}

	log.Printf("bookapp: html %v\n", string(data.Bytes()))
	return ba.writeResponse(data.Bytes())
}

func (ba *BookApp) doEdit(key string) *proc.Status {
	q := fmt.Sprintf("select * from book where title=\"%v\";", key)
	b, err := ba.query(q)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Query err %v\n", err), nil)
	}

	var books []dbd.Book
	err = json.Unmarshal(b, &books)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Marshall err %v\n", err), nil)
	}

	t, err := template.New("edit").Parse(`<h1>Editing {{.Title}}</h1>
<form action="/save/{{.Title}}" method="POST">
<div><textarea name="title" rows="20" cols="80">{{printf "%s" .Title}}</textarea></div>
<div><input type="submit" value="Save"></div>
</form>`)
	var data bytes.Buffer
	err = t.Execute(&data, books[0])
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Template err %v\n", err), nil)
	}

	log.Printf("bookapp: html %v\n", string(data.Bytes()))
	return ba.writeResponse(data.Bytes())
}

func (ba *BookApp) doSave(key string, title string) *proc.Status {
	q := fmt.Sprintf("update book SET title=\"%v\" where title=\"%v\";", title, key)
	_, err := ba.query(q)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Query err %v\n", err), nil)
	}
	return proc.MakeStatusErr("Redirect", "/book/view/")
}

func (ba *BookApp) Work() *proc.Status {
	log.Printf("work %v\n", ba.input)
	fd, err := ba.Open(path.Join(proc.PARENTDIR, proc.SHARED)+"/", np.OWRITE)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Open err %v\n", err), nil)
	}
	ba.pipefd = fd
	defer ba.Close(fd)

	switch ba.input[0] {
	case "view":
		return ba.doView()
	case "edit":
		return ba.doEdit(ba.input[1])
	case "save":
		return ba.doSave(ba.input[1], os.Args[3])
	default:
		return proc.MakeStatusErr("File not found", nil)
	}
}

func (ba *BookApp) Exit(status *proc.Status) {
	log.Printf("bookapp exit %v\n", status)
	ba.Exited(proc.GetPid(), status)
}
