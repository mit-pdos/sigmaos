package main

// read job description from stdin and submit the job's lambdas to ulambdad

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
	a.ExitDep = map[string]bool{}
	for _, dep := range split(l[5]) {
		a.ExitDep[dep] = false
	}
	fmt.Println("a ", a)
	return a, nil
}

func main() {
	clnt := fslib.MakeFsLib("submit")
	pctl := proc.MakeProcCtl(clnt, "submit")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		a, err := readLambda(scanner.Text())
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v: readLambda error %v\n", os.Args[0], err)
			os.Exit(1)
		}
		err = pctl.Spawn(a)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v: Spawn error %v\n", os.Args[0], err)
			os.Exit(1)
		}
	}
}
