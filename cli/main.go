package main

import (
	"bufio"
	"fmt"
	"os"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"strings"
)

func main() {
	
    // Initialize SigmaOS
    ts, err := NewTstateAll()
    if err != nil {
        fmt.Printf("Error initializing SigmaOS: %v\n", err)
        return
    }
    defer ts.Shutdown()

    fmt.Println("SigmaOS CLI initialized. Type 'help' for available commands.")

    scanner := bufio.NewScanner(os.Stdin)
    for {
        fmt.Print("SigmaOS> ")
        scanner.Scan()
        input := scanner.Text()
        args := strings.Fields(input)

        if len(args) == 0 {
            continue
        }

        switch args[0] {
        case "spawn":
            if len(args) < 2 {
                fmt.Println("Usage: spawn <proc_name> [args...]")
                continue
            }
            p := proc.NewProc(args[1], args[2:])
            err := ts.Spawn(p)
            if err != nil {
                fmt.Printf("Error spawning proc: %v\n", err)
            } else {
                fmt.Printf("Spawned proc with PID: %v\n", p.GetPid())
            }
        case "waitstart":
            if len(args) != 2 {
                fmt.Println("Usage: waitstart <pid>")
                continue
            }
            err := ts.WaitStart(sp.Tpid(args[1]))
            if err != nil {
                fmt.Printf("Error waiting for proc to start: %v\n", err)
            } else {
                fmt.Printf("Proc %s started\n", args[1])
            }
        case "waitexit":
            if len(args) != 2 {
                fmt.Println("Usage: waitexit <pid>")
                continue
            }
            status, err := ts.WaitExit(sp.Tpid(args[1]))
            if err != nil {
                fmt.Printf("Error waiting for proc to exit: %v\n", err)
            } else {
                fmt.Printf("Proc %s exited with status: %v\n", args[1], status.IsStatusOK())
                fmt.Printf("Exit message: %s\n", status.Msg())
            }
        case "exit":
            return
        case "help":
            fmt.Println("Available commands:")
            fmt.Println("  spawn <proc_name> [args...] - Spawn a new proc")
            fmt.Println("  waitstart <pid> - Wait for a proc to start")
            fmt.Println("  waitexit <pid> - Wait for a proc to exit")
            fmt.Println("  exit - Exit the CLI")
        default:
            fmt.Println("Unknown command. Type 'help' for available commands.")
        }
    }
}
