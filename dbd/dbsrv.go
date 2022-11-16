package dbd

import (
	"encoding/json"
	"log"
	"reflect"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	"sigmaos/debug"
)

type Server struct {
	db   *sql.DB
	rows *sql.Rows
}

func mkServer() (*Server, error) {
	s := &Server{}
	db, error := sql.Open("mysql", "sigma:sigmaos@/books")
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
	rows, err := s.db.Query(arg)
	if err != nil {
		return err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	count := len(columns)
	table := make([]map[string]interface{}, 0)
	valuePtrs := make([]interface{}, count)

	colTypes, err := rows.ColumnTypes()
	for i, s := range colTypes {
		switch s.ScanType().Kind() {
		case reflect.Int32:
			valuePtrs[i] = new(int32)
		default:
			valuePtrs[i] = new(sql.RawBytes)
		}
	}

	for rows.Next() {
		rows.Scan(valuePtrs...)
		entry := make(map[string]interface{})
		for i, col := range columns {
			var val interface{}
			valptr := valuePtrs[i]
			switch v := valptr.(type) {
			case *int32:
				val = *v
			case *sql.RawBytes:
				val = string(*v)
			default:
				log.Printf("unknown type %v\n", reflect.TypeOf(valptr))
			}
			entry[col] = val
		}
		table = append(table, entry)
	}
	debug.DPrintf("DBSRV", "doQuery: table (%d) %v\n", len(table), table)
	rb, err := json.Marshal(table)
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
