package netproxy

import (
	"net"
	"os"

	db "sigmaos/debug"
)

func listenerToFile(proxyListener net.Listener) (*os.File, error) {
	f, err := proxyListener.(*net.TCPListener).File()
	if err != nil {
		db.DFatalf("Error get TCP listener fd: %v", err)
		return nil, err
	}
	// Return the file object for the socket
	return f, nil
}

func connToFile(proxyConn net.Conn) (*os.File, error) {
	f, err := proxyConn.(*net.TCPConn).File()
	if err != nil {
		db.DFatalf("Error get TCP conn fd: %v", err)
		return nil, err
	}
	// Return the file object for the socket
	return f, nil
}

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
