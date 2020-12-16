package controler

import (
	"fmt"
)

type ControlerSrv struct {
}

type Args struct {
	Name string
}

func (s *ControlerSrv) Register(args *Args, reply *int) error {
	fmt.Printf("register %v\n", args)
	*reply = 1
	return nil
}
