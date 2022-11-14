package dbd

import (
	"encoding/json"
	"log"
	"reflect"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	"sigmaos/debug"
	"sigmaos/fs"
	np "sigmaos/ninep"
)

type Stream struct {
	db   *sql.DB
	rows *sql.Rows
}

func mkStream() (fs.File, *np.Err) {
	st := &Stream{}
	db, error := sql.Open("mysql", "sigma:sigmaos@/books")
	if error != nil {
		return nil, np.MkErrError(error)
	}
	st.db = db
	error = st.db.Ping()
	if error != nil {
		debug.DFatalf("open err %v\n", error)
	}
	log.Printf("Connected to db\n")
	return st, nil
}

// XXX wait on close before processing data?
func (st *Stream) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	rows, err := st.db.Query(string(b))
	if err != nil {
		return 0, np.MkErrError(err)
	}
	st.rows = rows
	return np.Tsize(len(b)), nil
}

// XXX incremental read
func (st *Stream) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if off > 0 {
		return nil, nil
	}
	if st.rows == nil {
		return nil, nil
	}
	defer st.rows.Close()
	columns, err := st.rows.Columns()
	if err != nil {
		return nil, np.MkErrError(err)
	}
	count := len(columns)
	table := make([]map[string]interface{}, 0)
	valuePtrs := make([]interface{}, count)

	colTypes, err := st.rows.ColumnTypes()
	for i, s := range colTypes {
		switch s.ScanType().Kind() {
		case reflect.Int32:
			valuePtrs[i] = new(int32)
		default:
			valuePtrs[i] = new(sql.RawBytes)
		}
	}

	for st.rows.Next() {
		st.rows.Scan(valuePtrs...)
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
	b, err := json.Marshal(table)
	if np.Tsize(len(b)) > cnt {
		np.MkErr(np.TErrInval, "too large")
	}
	if err != nil {
		return nil, np.MkErrError(err)
	}
	return b, nil
}
