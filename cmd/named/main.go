package main

import (
	"ulambda/memfsd"
)

func main() {
	fsd := memfsd.MakeFsd("named", ":1111", nil)
	fsd.Serve()
}
