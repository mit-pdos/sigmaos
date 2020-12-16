package lambda

import (
	"log"
	"net/rpc"

	"ulambda/controler"
)

type Controler struct {
	clnt    *rpc.Client
	lambdas map[string]LambdaF
}

// Create a new lambda
func (c *Controler) Fork(f LambdaF, args interface{}, reply interface{}) *Lambda {
	return &Lambda{c, f, args, reply}
}

// Register lambda `name` with controler so that controler knows we can run it
func (c *Controler) RegisterLambda(name string, f LambdaF) {
	args := controler.Args{name}
	var reply int
	err := c.clnt.Call("ControlerSrv.Register", args, &reply)
	if err != nil {
		log.Fatal("controler error:", err)
	}
}

// Make a persistent connection with controler
func ConnectToControler() *Controler {
	serverAddress := "localhost"
	client, err := rpc.Dial("tcp", serverAddress+":1234")
	if err != nil {
		log.Fatal("dialing:", err)
	}
	return &Controler{client, make(map[string]LambdaF)}
}
