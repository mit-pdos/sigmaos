package tsp

type Set []int

// Add naively trusts that number is not already in the set
// because TSP basically just removes numbers. It also
// adds the number to the existing set because it is a
// naive solution fit to TSP.
func (s *Set) Add(n int) {
	*s = append(*s, n)
}

// Del naively trusts that the number is already in the set.
// Del will copy the slice.
func (s Set) Del(n int) Set {
	out := make(Set, len(s)-1)
	i := 0
	for _, item := range s {
		if item != n {
			out[i] = item
			i++
		}
	}
	return out
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
