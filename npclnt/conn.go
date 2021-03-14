package npclnt

import (
	"bufio"
	"net"
	// "sync"
	//db "ulambda/debug"
	//np "ulambda/ninep"
)

type Chan struct {
	conn net.Conn
	br   *bufio.Reader
	bw   *bufio.Writer
}

func mkChan(addr string) (*Chan, error) {
	var err error
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	ch := &Chan{c,
		bufio.NewReaderSize(c, Msglen),
		bufio.NewWriterSize(c, Msglen)}
	return ch, nil
}
