package main

import (
	"os"
	"sigmaos/named"
)

// Usage: <named> address realmId pn [<peerId> <peers>]

func main() {
	named.Run(os.Args)
}
