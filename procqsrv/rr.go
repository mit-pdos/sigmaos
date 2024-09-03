package procqsrv

import (
	sp "sigmaos/sigmap"
)

type Realm struct {
	prev    *Realm
	next    *Realm
	id      sp.Trealm
	isEmpty bool
}

func newRealm(rid sp.Trealm) *Realm {
	return &Realm{
		prev:    nil,
		next:    nil,
		id:      rid,
		isEmpty: true,
	}
}

type RealmRR struct {
	realms map[sp.Trealm]*Realm
	head   *Realm
}

func NewRealmRR() *RealmRR {
	return &RealmRR{
		realms: make(map[sp.Trealm]*Realm),
		head:   nil,
	}
}

func (rr *RealmRR) allocRealm(rid sp.Trealm) *Realm {
	var r *Realm
	var ok bool
	if r, ok = rr.realms[rid]; !ok {
		r = newRealm(rid)
		rr.realms[rid] = r
	}
	return r
}

// Get the next realm, round-robin. If there is no previously un-seen realm
// with queued procs, return false.
func (rr *RealmRR) GetNextRealm(firstSeen sp.Trealm) (sp.Trealm, bool) {
	// There are no realms with queued procs, or the next realm has already been
	// seen.
	if rr.head == nil || rr.head.id == firstSeen {
		return sp.NO_REALM, false
	}
	nextRealm := rr.head
	rr.head = nextRealm.next
	return nextRealm.id, true
}

// Note that a realm's proc queue is not empty, and it shouldn't be considered
// for scheduling.
func (rr *RealmRR) RealmQueueNotEmpty(rid sp.Trealm) {
	r := rr.allocRealm(rid)
	// If queue was already not empty, bail out.
	if !r.isEmpty {
		return
	}
	r.isEmpty = false
	oldHead := rr.head
	// If there was no realm in the list, make this one the head and bail out
	if oldHead == nil {
		r.prev = r
		r.next = r
		rr.head = r
		return
	}
	// Insert r into the doubly linked list
	r.prev = oldHead.prev
	r.next = oldHead
	r.prev.next = r
	oldHead.prev = r
	// Make r the new head of the linked list
	rr.head = r
}

// Note that a realm's proc queue is empty, and it shouldn't be considered
// for scheduling anymore.
func (rr *RealmRR) RealmQueueEmpty(rid sp.Trealm) {
	r := rr.allocRealm(rid)
	r.isEmpty = true
	// If the empty-queued realm is the head, move the head along
	if rr.head == r {
		// If there is only one realm in the list (the head), unset the head
		if r.next == r {
			rr.head = nil
			return
		}
		// Move the head along
		rr.head = r.next
	}
	// Remove r from the doubly linked list
	prev := r.prev
	next := r.next
	prev.next = next
	next.prev = prev
	r.prev = r
	r.next = r
}
