package netsigma

import (
	"net"
	"os"

	db "sigmaos/debug"
	// "sigmaos/sessp"
)

func fdToUnixConn(fd int) (*net.UnixConn, error) {
	// Create a FileConn from the file descriptor
	conn, err := fdToConn(fd)
	if err != nil {
		db.DFatalf("Error make FileConn: %v", err)
	}
	return conn.(*net.UnixConn), nil
}

func fdToTCPConn(fd int) (*net.TCPConn, error) {
	// Create a FileConn from the file descriptor
	conn, err := fdToConn(fd)
	if err != nil {
		db.DFatalf("Error make FileConn: %v", err)
	}
	return conn.(*net.TCPConn), nil
}

func fdToConn(fd int) (net.Conn, error) {
	// Make the  FD into a Golang file object
	f := os.NewFile(uintptr(fd), "netproxy-conn")
	if f == nil {
		db.DFatalf("Error new file")
	}
	// Create a FileConn from the file
	conn, err := net.FileConn(f)
	if err != nil {
		db.DFatalf("Error make FileConn (%v): %v", fd, err)
	}
	return conn, nil
}
