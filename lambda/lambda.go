package lambda

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"strconv"
)

type LambdaF func(*Lambda) error

type Lambda struct {
	c      *Controler
	name   string
	Args   interface{}
	Reply  interface{}
	doneCh chan error
}

// Lambda creates another lambda
func (l *Lambda) Fork(name string, args interface{}, reply interface{}) *Lambda {
	err := l.c.ForkLambda(name, args)
	if err != nil {
		l := &Lambda{l.c, name, args, reply, make(chan error)}
		return l
	} else {
		return nil
	}
}

// Create a root lambda
func (c *Controler) Fork(name string, args interface{}, reply interface{}) *Lambda {
	err := c.ForkLambda(name, args)
	if err == nil {
		l := &Lambda{c, name, args, reply, make(chan error)}
		return l
	} else {
		return nil
	}
}

// Wait for lambda to run. If it returns Reply must have been filled in.
func (l *Lambda) Join() error {
	res := <-l.doneCh
	return res
}

type Controler struct {
	clnt *rpc.Client
	srv  *LambdaSrv
	port int
}

// Make a persistent connection with controler
func ConnectToControler() *Controler {
	serverAddress := "localhost"
	client, err := rpc.Dial("tcp", serverAddress+":1234")
	if err != nil {
		log.Fatal("dialing:", err)
	}

	// Start lambda server
	port := 2222
	fmt.Printf("port %v %v\n", port, strconv.Itoa(port))
	l, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	c := &Controler{client, &LambdaSrv{make(map[string]LambdaType)}, port}
	rpc.Register(c.srv)
	go c.RunSrv(l)

	return c
}

func (c *Controler) RunSrv(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("Accept error: ", err)
		}
		go rpc.ServeConn(conn)
	}
}
