package main

import (
  "fmt"
  "os"

  "sigmaos/gg"
)

func main() {
  if len(os.Args) < 5 {
    fmt.Fprintf(os.Stderr, "Usage: %v pid cwd target target_hash\n", os.Args[0])
    os.Exit(1)
  }
  tw, err := gg.MakeTargetWriter(os.Args[1:], false)
  if err != nil {
    fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
    os.Exit(1)
  }
  tw.Work()
  tw.Exit()
}
