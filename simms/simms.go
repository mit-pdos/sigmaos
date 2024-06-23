package simms

type Workload struct {
	t       *uint64
	app     *App
	clients *Clients
	stats   *Stats
}

func NewWorkload(t *uint64, app *App, clients *Clients) *Workload {
	return &Workload{
		t:       t,
		app:     app,
		clients: clients,
		stats:   NewStats(),
	}
}

func (d *Workload) Tick() {
	reqs := d.clients.Tick(*d.t)
	reps := d.app.Tick(reqs)
	d.stats.Update(reps)
}

func (d *Workload) Stats() *Stats {
	return d.stats
}
