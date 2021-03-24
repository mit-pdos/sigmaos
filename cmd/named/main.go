package main

import (
	db "ulambda/debug"
	"ulambda/memfsd"
)

func main() {
	db.Name("named")
	fsd := memfsd.MakeFsd(":1111", nil)
	fsd.Serve()
}
