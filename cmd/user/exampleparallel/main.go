package main

// import (
// 	"bufio"
// 	"fmt"

// 	//"log"
// 	"os"
// 	db "sigmaos/debug"
// 	"sigmaos/proc"
// 	"sigmaos/sigmaclnt"
// )

// takes pathname as argument
func main() {
	// fmt.Println(os.Args[1])
	// if len(os.Args) != 2 {
	// 	db.DFatalf("Should have 1 argument")
	// 	return
	// }

	// sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	// if err != nil {
	// 	db.DFatalf("NewSigmaClnt: error %v\n", err)
	// }
	// err = sc.Started()
	// if err != nil {
	// 	db.DFatalf("Started: error %v\n", err)
	// }
	// //	dir := sp.NAMED

	// //fn := filepath.Join(dir,os.Args[0])
	// fn := os.Args[1]
	// fmt.Println(fn, "hi")
	// rdr, err := sc.OpenReader(fn)
	// if err != nil {
	// 	db.DFatalf("Reader err: error %v\n", err)
	// }
	// scanner := bufio.NewScanner(rdr)
	// scanner.Split(bufio.ScanWords)
	// cnt := 0
	// for scanner.Scan() {
	// 	val := string(scanner.Bytes())
	// 	if val == "the" || val == "The" {
	// 		cnt += 1
	// 	}
	// }
	// fmt.Println(cnt)

	// // log.Printf("Hello world\n")

	// sc.ClntExit(proc.NewStatusInfo(proc.StatusOK, os.Args[1], cnt))
}
