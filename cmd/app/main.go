package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"ulambda/lambda"
)

type ReqArgs struct {
	N uint32
}

type ReqReply struct {
	N uint32
}

type ReqLambda struct {
	c *lambda.Controler
}

func (*ReqLambda) Lambda(args ReqArgs, reply *ReqReply) error {
	reply.N = args.N
	fmt.Printf("reply.N %v\n", reply.N)
	return nil
}

type ConnLambda struct {
	c *lambda.Controler
}

func (l *ConnLambda) ServeConn(conn net.Conn) {
	var hdr [4]byte
	_, err := io.ReadFull(conn, hdr[:])
	if err != nil {
		log.Fatal("Read error: ", err)
	}
	n := binary.BigEndian.Uint32(hdr[:])
	args := ReqArgs{n}
	child := l.c.Fork("ReqLambda.Lambda", args)

	var reply ReqReply
	err = child.Join(&reply)
	if err != nil {
		log.Fatal("Join error: ", err)
	}
	fmt.Printf("Join -> %v\n", reply.N)

	var rhdr [4]byte
	binary.BigEndian.PutUint32(rhdr[:], reply.N)
	_, err = conn.Write(rhdr[:])
	if err != nil {
		log.Fatal("Write error: ", err)
	}
	os.Exit(0)
}

type ConnArgs struct {
	N uint32
}

type ConnReply struct {
	N uint32
}

func (l *ConnLambda) Lambda(args ConnArgs, reply *ConnReply) error {
	s, err := net.Listen("tcp", ":1111")
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	for {
		conn, err := s.Accept()
		if err != nil {
			log.Fatal("Accept error: ", err)
		}
		go l.ServeConn(conn)
	}
	return nil
}

func main() {
	c := lambda.ConnectToControler()
	ll := &ConnLambda{c}
	c.RegisterLambda(ll)
	sl := &ReqLambda{c}
	c.RegisterLambda(sl)
	l := c.Fork("ConnLambda.Lambda", ConnArgs{})
	l.Join(&ConnReply{})
}
