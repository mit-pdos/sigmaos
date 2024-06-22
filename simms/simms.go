package simms

type Datacenter struct {
	t       *uint64
	app     *App
	clients *Clients
	stats   *Stats
}

func NewDatacenter(t *uint64, app *App, clients *Clients) *Datacenter {
	return &Datacenter{
		t:       t,
		app:     app,
		clients: clients,
		stats:   NewStats(),
	}
}

func (d *Datacenter) Tick() {
	reqs := d.clients.Tick(*d.t)
	reps := d.app.Tick(reqs)
	d.stats.Update(reps)
}

func (d *Datacenter) Stats() *Stats {
	return d.stats
}
