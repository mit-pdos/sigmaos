package lambda

import (
	"bytes"
	"encoding/gob"
	"log"

	"ulambda/controler"
)

//
// Controler client
//

// Fork lambda `name` so that controler can schedule it
func (c *Controler) ForkLambda(name string, args interface{}, id int64) error {
	b := new(bytes.Buffer)
	e := gob.NewEncoder(b)
	if err := e.Encode(args); err != nil {
		log.Fatal("Encode error:", err)
	}
	a := controler.ForkReq{name, b.Bytes(), id}
	var reply int
	err := c.clnt.Call("ControlerSrv.Fork", a, &reply)
	if err != nil {
		log.Fatal("ForkLambda error:", err)
	}
	return err
}

// Register lambda `name` with controler so that controler knows about this lambda
func (c *Controler) RegisterLambdaName(name string) {
	args := controler.RegisterReq{name, c.port}
	var reply int
	err := c.clnt.Call("ControlerSrv.Register", args, &reply)
	if err != nil {
		log.Fatal("controler error:", err)
	}
}

// Wait for lambda `id`
func (c *Controler) JoinLambda(name string, id int64) ([]byte, error) {
	args := controler.WaitReq{name, id}
	var reply controler.WaitReply
	err := c.clnt.Call("ControlerSrv.Join", args, &reply)
	if err != nil {
		log.Fatal("controler error:", err)
	}
	return reply.Reply, err
}
