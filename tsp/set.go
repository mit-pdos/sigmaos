package tsp

type Set []int

// Add naively trusts that number is not already in the set
// because TSP basically just removes numbers. It also
// adds the number to the existing set because it is a
// naive solution fit to TSP.
func (s *Set) Add(n int) {
	*s = append(*s, n)
}

// DelCopy naively trusts that the number is already in the set.
// DelCopy will copy the slice.
func (s *Set) DelCopy(n int) *Set {
	out := make(Set, len(*s)-1)
	i := 0
	for _, item := range *s {
		if item != n {
			out[i] = item
			i++
		}
	}
	return &out
}

// DelInPlace naively trusts that the number is already in the set.
func (s *Set) DelInPlace(n int) {
	// XXX Use Tombstones?
	index := s.has(n)
	*s = append((*s)[:index], (*s)[index+1:]...)
}

// has is an internal function which returns the index
// of the value, or -1 if it doesn't exist.
func (s *Set) has(n int) int {
	for i, v := range *s {
		if v == n {
			return i
		}
	}
	panic("Set.has called on set missing n")
}
