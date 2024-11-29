package main

import (
	"fmt"

	"net"
	"os"

	db "sigmaos/debug"
	"sigmaos/frame"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <socket>\n", os.Args[0])
		os.Exit(1)
	}

	socket, err := net.Listen("unix", os.Args[1])
	if err != nil {
		db.DFatalf("Listen: %v", err)
	}
	if err := os.Chmod(os.Args[1], 0777); err != nil {
		db.DFatalf("Err chmod sigmasocket: %v", err)
	}

	for {
		conn, err := socket.Accept()
		if err != nil {
			db.DFatalf("Error dialproxysrv Accept: %v", err)
			return
		}
		// Handle incoming connection
		go func(conn *net.UnixConn) {
			b, err := frame.ReadFrame(conn)
			if err != nil {
				db.DPrintf(db.DIALPROXYSRV_ERR, "Error Read PrincipalID frame: %v", err)
				return
			}
			db.DPrintf(db.ALWAYS, "frame: %v\n", string(b))
		}(conn.(*net.UnixConn))
	}

}
