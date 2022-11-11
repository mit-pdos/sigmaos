package main

import (
	"fmt"
	"os"

	// db "sigmaos/debug"
	"sigmaos/user"
)

//
// user login app, invoked by wwwd
//

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v args...\n", os.Args[0])
		os.Exit(1)
	}
	u, err := user.RunUserLogin(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	s := u.Login()
	u.Exit(s)
}
