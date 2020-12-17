package lambda

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"reflect"

	"ulambda/common"
)

type LambdaType struct {
	f LambdaF
	t reflect.Type
}

type LambdaSrv struct {
	lambdas map[string]LambdaType
}

func (s *LambdaSrv) Run(args *common.LambdaNet, reply *int) error {
	l, ok := s.lambdas[args.Name]
	if !ok {
		log.Fatal("Run: lookup failure:", args.Name)
	}

	fmt.Printf("Run at Srv %v t: %v\n", args, l.t)

	// space for argument
	a := reflect.New(l.t)
	ab := bytes.NewBuffer(args.Arg)
	ad := gob.NewDecoder(ab)
	if err := ad.Decode(a.Interface()); err != nil {
		log.Fatal("Run: decode failure: ", err)
	}
	// l.f()
	*reply = 0
	return nil
}
