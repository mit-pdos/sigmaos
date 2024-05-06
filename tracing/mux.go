package tracing

import (
	"net"
	"net/http"

	//	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	db "sigmaos/debug"
)

func NewHTTPMux() *TracedHTTPMux {
	return &TracedHTTPMux{
		http.NewServeMux(),
	}
}

type TracedHTTPMux struct {
	*http.ServeMux
}

func (tm *TracedHTTPMux) HandleFunc(pattern string, handler func(w http.ResponseWriter, r *http.Request)) {
	// Tag request with route, and wrap the request in a span context.
	tm.ServeMux.Handle(pattern, nil) //otelhttp.WithRouteTag(pattern+"/:name", http.HandlerFunc(handler)))
}

func (tm *TracedHTTPMux) Serve(l net.Listener) {
	db.DFatalf("%v", http.Serve(l, tm.ServeMux))
}
