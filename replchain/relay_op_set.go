package replchain

import (
	"sync"

	np "ulambda/ninep"
)

type RelayOpSet struct {
	mu      sync.Mutex
	entries map[np.Tsession]map[np.Tseqno][]*RelayOp
}

func MakeRelayOpSet() *RelayOpSet {
	s := &RelayOpSet{}
	s.entries = map[np.Tsession]map[np.Tseqno][]*RelayOp{}
	return s
}

func (s *RelayOpSet) Add(op *RelayOp) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[op.request.Session]; !ok {
		s.entries[op.request.Session] = map[np.Tseqno][]*RelayOp{}
	}
	if _, ok := s.entries[op.request.Session][op.request.Seqno]; !ok {
		s.entries[op.request.Session][op.request.Seqno] = []*RelayOp{}
	}
	s.entries[op.request.Session][op.request.Seqno] = append(s.entries[op.request.Session][op.request.Seqno], op)
}

// Atomically add another copy of this op if it's already in the set. Return
// true on success, and false otherwise.
func (s *RelayOpSet) AddIfDuplicate(op *RelayOp) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[op.request.Session]; !ok {
		return false
	}
	if _, ok := s.entries[op.request.Session][op.request.Seqno]; !ok {
		return false
	}
	s.entries[op.request.Session][op.request.Seqno] = append(s.entries[op.request.Session][op.request.Seqno], op)
	return true
}

// Remove & return all ops corresponding to this reply
func (s *RelayOpSet) Remove(reply *np.Fcall) []*RelayOp {
	s.mu.Lock()
	defer s.mu.Unlock()
	ops := []*RelayOp{}
	if session, ok := s.entries[reply.Session]; ok {
		if o, ok := session[reply.Seqno]; ok {
			ops = o
			delete(session, reply.Seqno)
		}
	}
	return ops
}

// Clear entry map and return all its contents
func (s *RelayOpSet) RemoveAll() []*RelayOp {
	s.mu.Lock()
	defer s.mu.Unlock()
	ops := s.getOpsL()
	s.entries = map[np.Tsession]map[np.Tseqno][]*RelayOp{}
	return ops
}

// Get all entries
func (s *RelayOpSet) GetOps() []*RelayOp {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getOpsL()
}

func (s *RelayOpSet) getOpsL() []*RelayOp {
	union := []*RelayOp{}
	for _, session := range s.entries {
		for _, ops := range session {
			union = append(union, ops...)
		}
	}
	return union
}
