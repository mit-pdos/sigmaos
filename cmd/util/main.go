package main

//
// helper utility for Python to notify the scheduler that
// a job has started, exited, &c.
//

import (
	"fmt"
	"os"
	"strings"

	"ulambda/fslib"
)

// XXX make input parsing more robust
func split(s string) []string {
	s = strings.TrimRight(s, "]")
	s = strings.TrimLeft(s, "[")
	if len(s) == 0 {
		return nil
	}
	s1 := strings.Split(s, " ")
	return s1
}

func splitPairs(s string) []fslib.PDep {
	ps := []fslib.PDep{}
	s = strings.TrimRight(s, "]")
	s = strings.TrimLeft(s, "[")
	if len(s) == 0 {
		return nil
	}
	s1 := strings.Split(s, " ")
	for _, e := range s1 {
		p := fslib.PDep{}
		e = strings.TrimRight(e, ")")
		e = strings.TrimLeft(e, "(")
		s1 := strings.Split(e, ";")
		p.Producer = s1[0]
		p.Consumer = s1[1]
		ps = append(ps, p)
	}
	return ps
}

func readLambda(line string) (*fslib.Attr, error) {
	l := strings.Split(line, ",")
	if len(l) != 6 {
		return nil, fmt.Errorf("not enough attributes")
	}
	a := &fslib.Attr{}
	a.Pid = l[0]
	a.Program = l[1]
	a.Args = split(l[2])
	a.Env = split(l[3])
	a.PairDep = splitPairs(l[4])
	a.ExitDep = split(l[5])
	fmt.Println("a ", a)
	return a, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %v exit <pid>\n", os.Args[0])
	os.Exit(1)
}

func main() {
	if len(os.Args) != 3 {
		usage()
	}
	clnt := fslib.MakeFsLib("util")
	cmd := os.Args[1]
	if cmd == "exit" {
		pid := os.Args[2]
		clnt.Exiting(pid)
	} else {
		usage()
	}
}
