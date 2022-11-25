package dbd

import (
	"log"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	"sigmaos/debug"
)

type Server struct {
	db   *sql.DB
	rows *sql.Rows
}

func mkServer(dbdaddr string) (*Server, error) {
	s := &Server{}
	db, error := sql.Open("mysql", "sigma:sigmaos@tcp("+dbdaddr+")/books")
	if error != nil {
		return nil, error
	}
	s.db = db
	error = s.db.Ping()
	if error != nil {
		debug.DFatalf("open err %v\n", error)
	}
	log.Printf("Connected to db\n")
	return s, nil
}

func (s *Server) doQuery(arg string, rep *[]byte) error {
	debug.DPrintf("DBSRV", "doQuery: %v\n", arg)
	rb, err := doQuery(s.db, arg)
	if err != nil {
		return err
	}
	*rep = rb
	return nil
}

func (s *Server) Query(arg string, rep *[]byte) error {
	err := s.doQuery(arg, rep)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) Exec(arg string, repl *[]byte) error {
	err := s.doQuery(arg, repl)
	if err != nil {
		return err
	}
	return nil
}
