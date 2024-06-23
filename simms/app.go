package simms

type App struct {
	ms *Microservice
}

func NewSingleTierApp(ms *Microservice) *App {
	return &App{
		ms: ms,
	}
}

func (a *App) Tick(reqs []*Request) []*Reply {
	return a.ms.Tick(reqs)
}
