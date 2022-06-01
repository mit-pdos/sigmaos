package main

import (
	"os"
	"ulambda/named"
)

// Usage: <named> address realmId <peerId> <peers>

func main() {
	named.Run(os.Args)
}
