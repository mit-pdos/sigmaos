package lambda

import (
	"fmt"
	"log"
	"strings"

	"ulambda/common"
)

type LambdaSrv struct {
	lambdas map[string]*LambdaReceiver
}

func (s *LambdaSrv) Run(args *common.LambdaReq, reply *common.LambdaReply) error {
	fmt.Printf("Run %v\n", args.Name)

	// split name into service and method
	dot := strings.LastIndex(args.Name, ".")
	serviceName := args.Name[:dot]
	methodName := args.Name[dot+1:]

	if lr, ok := s.lambdas[serviceName]; ok {
		reply.Reply, ok = lr.dispatch(methodName, args.Arg)
		return nil
	} else {
		log.Fatal("Run: lookup failure:", serviceName)
	}

	return nil
}
