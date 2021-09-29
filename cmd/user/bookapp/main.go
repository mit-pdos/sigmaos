package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"os"

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
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v pid args...\n", os.Args[0])
		os.Exit(1)
	}
	m, err := MakeBookApp(os.Args[1:])
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
	input string
	pipe  fs.FsObj
}

func MakeBookApp(args []string) (*BookApp, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeBookApp: too few arguments")
	}
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
	r.pid = args[0]
	r.input = args[1]
	r.pipe = pipe
	r.Started(r.pid)

	return r, nil
}

func (ba *BookApp) Work() string {
	log.Printf("work %v\n", ba.input)
	_, err := ba.pipe.Open(nil, np.OWRITE)
	if err != nil {
		return fmt.Sprintf("Open err %v\n", err)
	}
	defer ba.pipe.Close(nil, np.OWRITE)

	q := []byte("select * from book where author='Homer';")
	b, err := ba.ReadFile(dbd.DBD + "clone")
	if err != nil {
		return fmt.Sprintf("Clone err %v\n", err)
	}
	sid := string(b)
	err = ba.WriteFile(dbd.DBD+sid+"/query", q)
	if err != nil {
		return fmt.Sprintf("Query err %v\n", err)
	}

	b, err = ba.ReadFile(dbd.DBD + sid + "/data")
	if err != nil {
		return fmt.Sprintf("Query response err %v\n", err)
	}

	var books []dbd.Book
	err = json.Unmarshal(b, &books)
	if err != nil {
		return fmt.Sprintf("Marshall err %v\n", err)
	}

	t, err := template.New("test").Parse("<h1>Books</h1>{{.Title}} by {{.Author}}")
	if err != nil {
		return fmt.Sprintf("Template parse err %v\n", err)
	}

	var data bytes.Buffer
	err = t.Execute(&data, books[0])
	if err != nil {
		return fmt.Sprintf("Template err %v\n", err)
	}

	log.Printf("bookapp: html %v\n", string(data.Bytes()))

	_, err = ba.pipe.(fs.File).Write(nil, 0, data.Bytes(), np.NoV)
	if err != nil {
		return fmt.Sprintf("Pipe parse err %v\n", err)
	}

	ba.ExitFs("name/" + ba.pid)
	return "OK"
}

func (ba *BookApp) Exit(status string) {
	log.Printf("bookapp exit %v\n", status)
	ba.Exited(ba.pid, status)
}
