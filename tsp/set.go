package tsp

type Set []int

// Add naively trusts that number is not already in the set
// because TSP basically just removes numbers. It also
// adds the number to the existing set because it is a
// naive solution fit to TSP.
func (s *Set) Add(n int) {
	*s = append(*s, n)
}

// Del removes the supplied value, or returns the same
// set if the value is not in the set. Del will copy
// the slice.
func (s Set) Del(n int) Set {
	index := s.has(n)
	if index == -1 {
		cp := make(Set, len(s))
		_ = copy(cp, s)
		return cp
	}
	cp := make(Set, len(s))
	_ = copy(cp, s)
	return append(cp[:index], cp[index+1:]...)
}

// has is an internal function which returns the index
// of the value, or -1 if it doesn't exist.
func (s *Set) has(n int) int {
	for i, v := range *s {
		if v == n {
			return i
		}
	}
	return -1
}

func (s *Set) Has(n int) bool {
	for _, v := range *s {
		if v == n {
			return true
		}
	}
	return false
}
