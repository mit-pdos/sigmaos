package main

import (
  "fmt"
  "os"

  "ulambda/gg"
)

func main() {
  if len(os.Args) < 3 {
    fmt.Fprintf(os.Stderr, "Usage: %v pid thunk_hash\n", os.Args[0])
    os.Exit(1)
  }
  if len(os.Args) > 3 {
    fmt.Fprintf(os.Stderr, "More than one thunk hash passed! [%v]\n", os.Args)
    os.Exit(1)
  }
  ex, err := gg.MakeExecutor(os.Args[1:], true)
  if err != nil {
    fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
    os.Exit(1)
  }
  ex.Work()
  ex.Exit()
}
