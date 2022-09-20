package main

import (
	"os"
	"sigmaos/named"
)

// Usage: <named> address realmId <peerId> <peers>

func main() {
	named.Run(os.Args)
}
