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

func (w *Workload) Tick() {
	reqs := w.clients.Tick(*w.t)
	w.app.Tick(reqs)
}

func (w *Workload) GetStats() *ServiceStats {
	return w.app.GetStats()
}

func (w *Workload) RecordStats(window int) {
	w.app.GetStats().RecordStats(window)
}

func (w *Workload) StopRecordingStats() {
	w.app.GetStats().RecordStats(0)
}
