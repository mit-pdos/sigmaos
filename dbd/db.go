package dbd

import (
	"fmt"
	"strconv"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	// db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/protdevsrv"
	"sigmaos/user"
)

type Book struct {
	Author string
	Price  string
	Title  string
}

//
// mysql client exporting a database server through the file system
// interface, modeled after
// http://man.cat-v.org/plan_9_contrib/4/mysqlfs
//

func initDb() {
	db, err := sql.Open("mysql", "sigma:sigmaos@/books")
	if err != nil {
		panic(err.Error())
	}

	rows, err := db.Query("SELECT * FROM user")
	if err != nil {
		panic(err.Error())
	}
	defer rows.Close()
	if rows.Next() {
		return
	}

	for i := 0; i <= 500; i++ {
		u := "u_" + strconv.Itoa(i)
		p := user.MkPassword(u)
		sql := fmt.Sprintf("INSERT INTO user(username, password) VALUES ('%s', '%s')", u, p)
		_, err := db.Exec(sql)
		if err != nil {
			panic(err.Error())
		}
	}
}

func RunDbd() error {
	// seccomp.LoadFilter()  // sanity check: if enabled we want dbd to fail
	initDb()
	s, err := mkServer()
	if err != nil {
		return err
	}
	pds := protdevsrv.MakeProtDevSrv(np.DB, s)
	return pds.RunServer()
}
