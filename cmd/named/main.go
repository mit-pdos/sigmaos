package main

import (
	"ulambda/memfsd"
)

func main() {
	fsd := memfsd.MakeFsd(":1111", nil)
	fsd.Serve()
}
