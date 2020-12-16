package controler

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
)

type Controler struct {
	srv *ControlerSrv
	l   net.Listener
}

func MakeControler() *Controler {
	c := &Controler{}
	c.srv = &ControlerSrv{}
	l, err := net.Listen("tcp", ":1234")
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	rpc.Register(c.srv)
	c.l = l
	return c
}

func (c *Controler) Run() {
	for {
		conn, err := c.l.Accept()
		if err != nil {
			log.Fatal("Accept error: ", err)
		}
		go rpc.ServeConn(conn)
	}
}

func (c *Controler) Stop() {
	fmt.Printf("stop\n")
}
