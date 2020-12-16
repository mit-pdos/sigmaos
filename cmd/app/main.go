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

type Args struct {
	n uint32
}

type Reply struct {
	n uint32
}

func shortLambda(l *lambda.Lambda) error {
	a := l.Args.(Args)
	r := l.Reply.(*Reply)
	r.n = a.n
	fmt.Printf("run short lambda: res %v\n", r.n)
	return nil
}

func ServeConn(l *lambda.Lambda, conn net.Conn) {
	var hdr [4]byte
	_, err := io.ReadFull(conn, hdr[:])
	if err != nil {
		log.Fatal("Read error: ", err)
	}
	n := binary.BigEndian.Uint32(hdr[:])
	args := Args{n}
	var reply Reply
	child := l.Fork(shortLambda, args, &reply)

	err = child.Join()
	if err != nil {
		log.Fatal("Join error: ", err)
	}
	r := child.Reply.(*Reply)
	var rhdr [4]byte
	binary.BigEndian.PutUint32(rhdr[:], r.n)
	_, err = conn.Write(rhdr[:])
	if err != nil {
		log.Fatal("Write error: ", err)
	}
	// os.Exit(0)
}

func longLambda(l *lambda.Lambda) error {
	fmt.Printf("run longLambda\n")
	s, err := net.Listen("tcp", ":1235")
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	for {
		conn, err := s.Accept()
		if err != nil {
			log.Fatal("Accept error: ", err)
		}
		go ServeConn(l, conn)
	}
	return nil
}

func main() {
	controler := lambda.ConnectToControler()
	controler.RegisterLambda("longLambda", longLambda)
	controler.RegisterLambda("shortLamabda", shortLambda)
	l := controler.Fork(longLambda, nil, nil)
	l.Join()
}
