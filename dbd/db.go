package dbd

import (
	"crypto/sha256"
	"fmt"
	"strconv"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fslibsrv"
	np "sigmaos/ninep"
)

//
// mysql client exporting a database server through the file system
// interface, modeled after
// http://man.cat-v.org/plan_9_contrib/4/mysqlfs
//

const (
	DBD = "name/db/~ip/"
)

type Book struct {
	Author string
	Price  string
	Title  string
}

type User struct {
	Username string
	Password string
}

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
		s := strconv.Itoa(i)
		u := "Cornell_" + s
		p := ""
		for j := 0; j < 10; j++ {
			p += s
		}
		sum := sha256.Sum256([]byte(p))
		sql := fmt.Sprintf("INSERT INTO user(username, password) VALUES ('%s', '%x')", u, sum)
		_, err := db.Exec(sql)
		if err != nil {
			panic(err.Error())
		}
	}
}

func RunDbd() {
	// seccomp.LoadFilter()  // sanity check: if enabled we want dbd to fail
	initDb()
	mfs, _, _, error := fslibsrv.MakeMemFs(np.DB, "dbd")
	if error != nil {
		db.DFatalf("StartMemFs %v\n", error)
	}
	err := dir.MkNod(ctx.MkCtx("", 0, nil), mfs.Root(), "clone", makeClone(nil, mfs.Root()))
	if err != nil {
		db.DFatalf("MakeNod clone failed %v\n", err)
	}
	mfs.Serve()
	mfs.Done()
}
