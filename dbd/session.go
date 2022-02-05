package dbd

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	"ulambda/dir"
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

type Clone struct {
	fs.FsObj
}

func makeClone(ctx fs.CtxI, parent fs.Dir) fs.FsObj {
	i := inode.MakeInode(ctx, np.DMDEVICE, parent)
	return &Clone{i}
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
	db, err := sql.Open("mysql", "sigma:sigmaos@/books")
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		log.Fatalf("FATAL open err %v\n", err)
	}
	log.Printf("Connected to db\n")

	s := &Session{}
	s.Inode = inode.MakeInode(nil, 0, nil)
	s.id = strconv.Itoa(int(s.Inode.Inum()))
	s.db = db

	// create directory for session
	di := inode.MakeInode(nil, np.DMDIR, c.Parent())
	d := dir.MakeDir(di)
	err = dir.MkNod(ctx, c.Parent(), s.id, d)
	if err != nil {
		log.Fatalf("FATAL MkNod err %v\n", err)
	}
	err = dir.MkNod(ctx, d, "ctl", s) // put ctl file into session dir
	if err != nil {
		log.Fatalf("FATAL MkNod err %v\n", err)
	}

	// make query file
	q := &Query{}
	q.db = db
	q.Inode = inode.MakeInode(nil, 0, d)
	dir.MkNod(ctx, d, "query", q)
	dir.MkNod(ctx, d, "data", q)

	return s, nil
}

func (s *Session) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, error) {
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
	return b, nil
}
