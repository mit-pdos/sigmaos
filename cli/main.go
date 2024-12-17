package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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

	currentDir := "name/"
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("SigmaOS:%s> ", currentDir)
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
		case "ls":
			sts, err := ts.GetDir(currentDir)
			if err != nil {
				fmt.Printf("Error listing directory: %v\n", err)
				continue
			}
			for _, filename := range sp.Names(sts) {
				fmt.Println(filename)
			}
		case "cd":
			if len(args) != 2 {
				fmt.Println("Usage: cd <directory>")
				continue
			}
			newDir := args[1]
			if !filepath.IsAbs(newDir) {
				newDir = filepath.Join(currentDir, newDir)
			}
			_, err := ts.GetDir(newDir)
			if err != nil {
				fmt.Printf("Error changing directory: %v\n", err)
				continue
			}
			currentDir = newDir
		case "pwd":
			fmt.Println(currentDir)
		case "putfile":
			if len(args) != 3 {
				fmt.Println("Usage: putfile <filename> <content>")
				continue
			}
			filename := filepath.Join(currentDir, args[1])
			content := []byte(args[2])
			_, err := ts.PutFile(filename, 0777, sp.OWRITE, content)
			if err != nil {
				fmt.Printf("Error writing file: %v\n", err)
			} else {
				fmt.Printf("File %s created successfully\n", args[1])
			}
		case "getfile":
			if len(args) != 2 {
				fmt.Println("Usage: getfile <filename>")
				continue
			}
			filename := filepath.Join(currentDir, args[1])
			data, err := ts.GetFile(filename)
			if err != nil {
				fmt.Printf("Error reading file: %v\n", err)
			} else {
				fmt.Printf("File contents: %s\n", string(data))
			}
		case "exit":
			return
		case "help":
			fmt.Println("Available commands:")
			fmt.Println("  spawn <proc_name> [args...] - Spawn a new proc")
			fmt.Println("  waitstart <pid> - Wait for a proc to start")
			fmt.Println("  waitexit <pid> - Wait for a proc to exit")
			fmt.Println("  ls - List contents of the current directory")
			fmt.Println("  cd <directory> - Change current directory")
			fmt.Println("  pwd - Print current working directory")
			fmt.Println("  putfile <filename> <content> - Create and write to a file")
			fmt.Println("  getfile <filename> - Read and display file contents")
			fmt.Println("  exit - Exit the CLI")
		default:
			fmt.Println("Unknown command. Type 'help' for available commands.")
		}
	}
}
