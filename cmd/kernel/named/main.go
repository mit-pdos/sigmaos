package main

import (
	"os"
	"ulambda/named"
)

// Usage: <named> address realmId <peerId> <peers> <pprofPath> <utilPath>

func main() {
	named.Run(os.Args)
}
