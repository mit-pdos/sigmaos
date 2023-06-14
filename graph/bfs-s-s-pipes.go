package graph

import (
	"encoding/binary"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

/*func BfsSinglePipes(g *Graph, n1 int, n2 int) (*[]int, error) {
	ts := time.Now()
	path, err := g.BfsSingleChannels(n1, n2)
	te := time.Now()
	printTime(ts, te, "BfsSingleChannels ran")
	return path, err
}*/

// This is basically trying to be a channel; writes append to the end,
// reads pop from the front.
// TODO: Keep track of where the head is
// TODO: Pop when reading
func writeInt(val int, thread Thread) error {
	db.DPrintf(DEBUG_GRAPH, "Writing %v", val)
	// XXX HANGS ON THIS OPEN
	fd, err := thread.Open(thread.Pipe, sp.OWRITE)
	db.DPrintf(DEBUG_GRAPH, "Writing Opened")
	if err != nil {
		return err
	}
	b := make([]byte, 4)
	binary.LittleEndian.PutUint64(b, uint64(val))
	db.DPrintf(DEBUG_GRAPH, "Writing bytes %v", b)
	if _, err = thread.Write(fd, b); err != nil {
		return err
	}
	return thread.Close(fd)
}

func readInt(thread Thread) (int, error) {
	db.DPrintf(DEBUG_GRAPH, "Reading")
	fd, err := thread.Open(thread.Pipe, sp.OREAD)
	db.DPrintf(DEBUG_GRAPH, "Reading Opened")
	b, err := thread.Read(fd, 4)
	if err != nil {
		return -1, err
	}
	db.DPrintf(DEBUG_GRAPH, "Read val %v: %v", b, binary.LittleEndian.Uint64(b))
	return int(binary.LittleEndian.Uint64(b)), thread.Close(fd)
}

// BfsSinglePipes is a single-threaded, distributed, iterative breadth first search
// between two given nodes which works continuously via pipes.
func (g *Graph) BfsSinglePipes(n1 int, n2 int, thread Thread) (*[]int, error) {
	if n1 == n2 {
		return &[]int{n1}, nil
	}
	if n1 > g.NumNodes-1 || n2 > g.NumNodes-1 || n1 < 0 || n2 < 0 {
		return nil, ERR_SEARCH_OOR
	}
	// p[index] gives the parent node of index
	p := make([]int, g.NumNodes)
	for i := range p {
		p[i] = NOT_VISITED
	}
	p[n1] = n1
	// Continual Set
	if err := writeInt(n1, thread); err != nil {
		return nil, err
	}
	// XXX END CONDITION
	for i := 0; i < 10000; i++ {
		index, err := readInt(thread)
		if err != nil {
			return nil, err
		}
		adj := g.GetNeighbors(index)
		for _, a := range *adj {
			if p[a] == NOT_VISITED {
				if err := writeInt(a, thread); err != nil {
					return nil, err
				}
				p[a] = index
				if a == n2 {
					// Return the shortest path from n1 to n2
					return findPath(&p, n1, n2), nil
				}
			}
		}
	}
	return nil, ERR_NOPATH
}
