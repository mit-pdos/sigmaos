package lambda

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"reflect"

	// "ulambda/common"
	"ulambda/controler"
)

//
// Controler client
//

// Fork lambda `name` so that controler can schedule it
func (c *Controler) ForkLambda(name string, args interface{}) error {
	b := new(bytes.Buffer)
	e := gob.NewEncoder(b)
	if err := e.Encode(args); err != nil {
		log.Fatal("Encode error:", err)
	}
	a := controler.ForkArgs{name, b.Bytes()}
	fmt.Printf("ForkLambda: arg %v %v\n", a, reflect.TypeOf(args))
	var reply int
	err := c.clnt.Call("ControlerSrv.Fork", a, &reply)
	if err != nil {
		log.Fatal("ForkLambda error:", err)
	}
	return err
}

// Register lambda `name` with controler so that controler knows about this lambda
func (c *Controler) RegisterLambda(name string, l LambdaF, arg interface{}) {
	c.srv.lambdas[name] = LambdaType{l, reflect.TypeOf(arg)}
	args := controler.RegisterArgs{name, c.port}
	var reply int
	err := c.clnt.Call("ControlerSrv.Register", args, &reply)
	if err != nil {
		log.Fatal("controler error:", err)
	}
}
