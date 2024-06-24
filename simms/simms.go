package simms

type Workload struct {
	t       *uint64
	app     *App
	clients *Clients
	stats   *WorkloadStats
}

func NewWorkload(t *uint64, app *App, clients *Clients) *Workload {
	return &Workload{
		t:       t,
		app:     app,
		clients: clients,
		stats:   NewWorkloadStats(),
	}
}

func (d *Workload) Tick() {
	reqs := d.clients.Tick(*d.t)
	reps := d.app.Tick(reqs)
	d.stats.Tick(reps)
}

func (d *Workload) Stats() *WorkloadStats {
	return d.stats
}
