package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"ulambda/controler"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("missing argument")
	}
	c := controler.MakeControler()
	go c.Run()
	out, err := exec.Command(os.Args[1]).CombinedOutput()
	fmt.Printf("exec: %s\n", out)
	if err != nil {
		log.Fatal("Wait error: ", err)
	}
	c.Stop()
}
