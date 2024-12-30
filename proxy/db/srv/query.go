package srv

import (
	"database/sql"
	"encoding/json"
	"log"
	"reflect"

	_ "github.com/go-sql-driver/mysql"

	"sigmaos/debug"
)

func doQuery(db *sql.DB, arg string) ([]byte, error) {
	debug.DPrintf(debug.DB, "doQuery: %v\n", arg)
	rows, err := db.Query(arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	count := len(columns)
	table := make([]map[string]interface{}, 0)
	valuePtrs := make([]interface{}, count)

	colTypes, err := rows.ColumnTypes()
	for i, s := range colTypes {
		switch s.ScanType().Kind() {
		case reflect.Int32:
			valuePtrs[i] = new(int32)
		case reflect.Int64:
			valuePtrs[i] = new(int64)
		case reflect.Float32:
			valuePtrs[i] = new(float32)
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
			case *int64:
				val = *v
			case *float32:
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
	debug.DPrintf(debug.DB, "doQuery: table (%d) %v\n", len(table), table)
	rb, err := json.Marshal(table)
	if err != nil {
		return nil, err
	}
	return rb, nil
}
