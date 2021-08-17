package main

// read job description from stdin and submit the job's lambdas to ulambdad

import (
	"bufio"
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

func main() {
	clnt := fslib.MakeFsLib("submit")
	sctl := depproc.MakeDepProcCtl(clnt, depproc.DEFAULT_JOB_ID)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		a, err := readLambda(scanner.Text())
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v: readLambda error %v\n", os.Args[0], err)
			os.Exit(1)
		}
		err = sctl.Spawn(a)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v: Spawn error %v\n", os.Args[0], err)
			os.Exit(1)
		}
	}
}
