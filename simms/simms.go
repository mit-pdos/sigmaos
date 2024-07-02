package simms

type Workload struct {
	t       *uint64
	app     *App
	clients *Clients
}

func NewWorkload(t *uint64, app *App, clients *Clients) *Workload {
	return &Workload{
		t:       t,
		app:     app,
		clients: clients,
	}
}

func (d *Workload) Tick() {
	reqs := d.clients.Tick(*d.t)
	d.app.Tick(reqs)
}

func (d *Workload) GetStats() *ServiceStats {
	return d.app.GetStats()
}

func (d *Workload) RecordStats(window int) {
	d.app.GetStats().RecordStats(window)
}

func (d *Workload) StopRecordingStats() {
	d.app.GetStats().RecordStats(0)
}
