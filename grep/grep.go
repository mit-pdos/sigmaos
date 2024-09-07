package grep

//
// a grep application for MapReduce.
//

import (
	"bufio"
	"strconv"

	"sigmaos/mr"
)

var words = []string{"JavaScript", "Java", "PHP", "Python", "C#", "C++",
	"Ruby", "CSS", "Objective-C", "Perl",
	"Scala", "Haskell", "MATLAB", "Clojure", "Groovy"}

var target map[string]struct{}

func init() {
	target = make(map[string]struct{})
	for _, w := range words {
		target[w] = struct{}{}
	}
}

func Map(filename string, scanner *bufio.Scanner, emit mr.EmitT) error {
	for scanner.Scan() {
		w := scanner.Text()
		if _, ok := target[w]; ok {
			if err := emit(w, "1"); err != nil {
				return err
			}
		}
	}
	return nil
}

func Reduce(key string, values []string, emit mr.EmitT) error {
	if err := emit(key, strconv.Itoa(len(values))); err != nil {
		return err
	}
	return nil
}
