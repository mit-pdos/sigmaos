package crash

import (
	"encoding/json"

	"sigmaos/proc"
)

type TeventMap struct {
	Evs map[Tselector]Tevent `json:"evs"`
}

func NewTeventMap() *TeventMap {
	return &TeventMap{Evs: make(map[Tselector]Tevent)}
}

func NewTeventMapOne(e Tevent) *TeventMap {
	em := NewTeventMap()
	em.Evs[Tselector(e.Label)] = e
	return em
}

func (em *TeventMap) Events2String() (string, error) {
	b, err := json.Marshal(em)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (em *TeventMap) Insert(e Tevent) {
	em.Evs[Tselector(e.Label)] = e
}

func (em *TeventMap) Lookup(l Tselector) (Tevent, bool) {
	e, ok := em.Evs[l]
	return e, ok
}

func (em *TeventMap) Merge(em0 *TeventMap) {
	for k, v := range em0.Evs {
		em.Evs[k] = v
	}
}

func (em0 *TeventMap) Filter(l Tselector) *TeventMap {
	em1 := NewTeventMap()
	for k, v := range em0.Evs {
		if k == l {
			em1.Evs[k] = v
		}
	}
	return em1
}

func (em *TeventMap) AppendEnv(p *proc.Proc) error {
	s, err := em.Events2String()
	if err != nil {
		return err
	}
	p.AppendEnv(proc.SIGMAFAIL, s)
	return nil
}
