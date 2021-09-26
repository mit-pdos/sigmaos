package dbd

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

type Clone struct {
	*inode.Inode
}

type Session struct {
	*inode.Inode
	id string
	db *sql.DB
}

type Query struct {
	*inode.Inode
	db   *sql.DB
	rows *sql.Rows
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
	s.Inode = inode.MakeInode("", 0, nil)
	s.id = strconv.Itoa(int(s.Inode.Inum()))
	s.db = db

	// create directory for session
	d := makeDir(nil, []string{s.id}, np.DMDIR, c.Parent().(*Dir))
	s.Inode.SetParent(d)
	c.Parent().(*Dir).create(s.id, d)
	d.create("ctl", s) // put ctl file into session dir

	// make query file
	q := &Query{}
	q.db = db
	q.Inode = inode.MakeInode("", 0, d)
	d.create("query", q)
	d.create("data", q)

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

// XXX wait on close before processing data?
func (q *Query) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	log.Printf("query: %v", string(b))
	rows, err := q.db.Query(string(b))
	if err != nil {
		return 0, err
	}
	q.rows = rows
	return np.Tsize(len(b)), nil
}

// XXX incremental read
func (q *Query) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
	if off > 0 {
		return nil, nil
	}
	defer q.rows.Close()
	columns, err := q.rows.Columns()
	if err != nil {
		return nil, err
	}
	count := len(columns)
	table := make([]map[string]interface{}, 0)
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)
	for q.rows.Next() {
		for i := 0; i < count; i++ {
			valuePtrs[i] = &values[i]
		}
		q.rows.Scan(valuePtrs...)
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
		table = append(table, entry)
	}
	b, err := json.Marshal(table)
	if np.Tsize(len(b)) > cnt {
		return nil, fmt.Errorf("too large")
	}
	if err != nil {
		return nil, err
	}
	log.Printf(string(b))
	return b, nil
}
