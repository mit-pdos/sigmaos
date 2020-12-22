package proc

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
)

type Controler struct {
	srv *ControlerSrv
}

func MakeControler() *Controler {
	c := &Controler{}
	c.srv = mkControlerSrv()
	l, err := net.Listen("tcp", ":1234")
	if err != nil {
		log.Fatal("Listen error:", err)
	}
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

func (c *Controler) Stop() {
	fmt.Printf("stop\n")
}
