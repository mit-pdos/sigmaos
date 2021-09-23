package dbd

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	"ulambda/fs"
	np "ulambda/ninep"
)

type Clone struct {
	*Obj
}

type Session struct {
	*Obj
	id string
	db *sql.DB
}

type Query struct {
	*Obj
	db *sql.DB
}

func (c *Clone) Open(ctx fs.CtxI, m np.Tmode) (fs.FsObj, error) {
	log.Printf("session open %v\n", c)
	db, err := sql.Open("mysql", "sigma:sigmaos@/books")
	if err != nil {
		return nil, err
	}
	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}
	log.Printf("Connected\n")

	s := &Session{}
	s.Obj = makeObj(c.db, nil, 0, nil)
	s.id = strconv.Itoa(int(s.Obj.id))
	s.path = []string{s.id, "ctl"}
	s.db = db

	// create directory for session
	d := makeDir(c.Obj.db, []string{s.id}, np.DMDIR, c.p)
	s.Obj.p = d
	c.p.create(s.id, d)
	d.create("ctl", s) // put ctl file into session dir

	// make query file
	q := &Query{}
	q.db = db
	q.Obj = makeObj(c.db, []string{s.id, "query"}, 0, d)
	d.create("query", q)

	return s, nil
}

func (s *Session) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	log.Printf("read session %v off %v\n", s, off)
	if off > 0 {
		return nil, nil
	}
	return []byte(s.id), nil
}

func (s *Session) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	return 0, fmt.Errorf("not supported")
}

func (q *Query) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	return nil, fmt.Errorf("not supported")
}

// XXX wait on close before processing data?
func (q *Query) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	log.Printf("query: %v", string(b))
	rows, err := q.db.Query(string(b))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	count := len(columns)
	tableData := make([]map[string]interface{}, 0)
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)
	for rows.Next() {
		for i := 0; i < count; i++ {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)
		entry := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			entry[col] = v
		}
		tableData = append(tableData, entry)
	}
	jsonData, err := json.Marshal(tableData)
	if err != nil {
		return 0, err
	}
	log.Printf(string(jsonData))
	return np.Tsize(len(b)), nil
}
