package main

//
// helper utility for Python to notify the scheduler that
// a job has started, exited, &c.
//

import (
	"fmt"
	"os"
	"strings"

	"ulambda/depproc"
	"ulambda/fslib"
	"ulambda/proc"
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

func splitPairs(s string) map[string]bool {
	ps := map[string]bool{}
	s = strings.TrimRight(s, "]")
	s = strings.TrimLeft(s, "[")
	if len(s) == 0 {
		return nil
	}
	s1 := strings.Split(s, " ")
	for _, e := range s1 {
		e = strings.TrimRight(e, ")")
		e = strings.TrimLeft(e, "(")
		s1 := strings.Split(e, ";")
		ps[s1[0]] = false
	}
	return ps
}

func readLambda(line string) (*depproc.Task, error) {
	l := strings.Split(line, ",")
	if len(l) != 6 {
		return nil, fmt.Errorf("not enough attributes")
	}
	t := depproc.MakeTask()
	a := &proc.Proc{}
	a.Pid = l[0]
	a.Program = l[1]
	a.Args = split(l[2])
	a.Env = split(l[3])
	t.Proc = a
	t.Dependencies = &depproc.Deps{}
	t.Dependencies.StartDep = splitPairs(l[4])
	t.Dependencies.ExitDep = map[string]bool{}
	for _, dep := range split(l[5]) {
		t.Dependencies.ExitDep[dep] = false
	}
	fmt.Println("a ", t)
	return t, nil
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
	sctl := depproc.MakeDepProcCtl(clnt, depproc.DEFAULT_JOB_ID)
	cmd := os.Args[1]
	if cmd == "exit" {
		pid := os.Args[2]
		sctl.Exited(pid)
	} else {
		usage()
	}
}
