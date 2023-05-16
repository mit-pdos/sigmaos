package main

import (
	"os"

	"sigmaos/namedv1"
)

// Usage: <named> address realmId pn [<peerId> <peers>]

func main() {
	namedv1.Run(os.Args)
}
