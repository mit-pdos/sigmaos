package netproxytrans

import (
	"net"
	"os"
	"time"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
)

func ListenerToFile(proxyListener net.Listener) (*os.File, error) {
	f, err := proxyListener.(*net.TCPListener).File()
	if err != nil {
		db.DFatalf("Error get TCP listener fd: %v", err)
		return nil, err
	}
	// Return the file object for the socket
	return f, nil
}

func ConnToFile(proxyConn net.Conn) (*os.File, error) {
	start := time.Now()
	c := proxyConn.(*net.TCPConn)
	db.DPrintf(db.NETPROXY_LAT, "Dial ConnToFile typecast latency: %v", time.Since(start))
	start = time.Now()
	f, err := c.File()
	dur := time.Since(start)
	db.DPrintf(db.NETPROXY_LAT, "Dial ConnToFile File[%v] latency: %v", f.Fd(), dur)
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

func ConstructSocketControlMsg(proxiedFile *os.File) []byte {
	fd := int(proxiedFile.Fd())
	return unix.UnixRights(fd)
}

func ParseReturnedConn(oob []byte) (*net.TCPConn, error) {
	// Sanity check
	if len(oob) == 0 {
		db.DPrintf(db.ERROR, "Error oob len 0")
		db.DFatalf("Error oob len 0")
	}
	scma, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		db.DFatalf("Error parse socket control message: %v", err)
	}
	fds, err := unix.ParseUnixRights(&scma[0])
	if err != nil || len(fds) != 1 {
		db.DFatalf("Error parse unix rights: len %v err %v", len(fds), err)
	}
	db.DPrintf(db.NETPROXYCLNT, "got socket fd %v", fds[0])
	return fdToTCPConn(fds[0])
}
