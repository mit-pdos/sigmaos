package simms

import (
	db "sigmaos/debug"
)

type DB struct {
	svc *Service
}

type Memcache struct {
	svc *Service
}

func NewDB(t *uint64, dbp *Params) *DB {
	return &DB{
		svc: NewService(t, dbp),
	}
}

func NewMemcache(t *uint64, mcp *Params) *Memcache {
	return &Memcache{
		svc: NewService(t, mcp),
	}
}

type Microservice struct {
	svc *Service
	mc  *Memcache
	db  *DB
}

type App struct {
	wfe *Microservice
}

func NewMicroservice(t *uint64, msp *Params, mc *Memcache, db *DB) *Microservice {
	return &Microservice{
		svc: NewService(t, msp),
		mc:  mc,
		db:  db,
	}
}

func (m *Microservice) Tick(reqs []*Request) []*Reply {
	if m.mc != nil {
		db.DFatalf("Unimplemented: microservice with memcache")
	}
	if m.db != nil {
		db.DFatalf("Unimplemented: microservice with db")
	}
	// TODO: request type (compute vs fetch)
	// TODO: request data (fetch % chance)
	return m.svc.Tick(reqs)
}

func NewApp(wfe *Microservice) *App {
	return &App{
		wfe: wfe,
	}
}

func (a *App) Tick(reqs []*Request) []*Reply {
	return a.wfe.Tick(reqs)
}
