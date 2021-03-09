package main

import (
	"ulambda/npux"
)

func main() {
	npux := npux.MakeNpUx("/tmp")
	npux.Serve()
}
