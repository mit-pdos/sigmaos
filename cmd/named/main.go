package main

import (
	db "ulambda/debug"
	"ulambda/memfsd"
)

func main() {
	db.Name("sharder")
	fsd := memfsd.MakeFsd(":1111", nil)
	fsd.Serve()
}
