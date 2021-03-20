package main

import (
	db "ulambda/debug"
	"ulambda/memfsd"
)

func main() {
	fsd := memfsd.MakeFsd(db.Name("named"), ":1111", nil)
	fsd.Serve()
}
