package wess

import (
	"net/http"

	"github.com/gildas/go-core"
	"github.com/gildas/go-logger"
	"github.com/gorilla/mux"
)

// healthRoutes adds the Health Routes to the given Router
func (server *Server) healthRoutes(router *mux.Router) {
	router.Methods("GET").Path("/liveness").Handler(healthHandler(server, "liveness"))
	router.Methods("GET").Path("/readiness").Handler(healthHandler(server, "readiness"))
}

// healthHandler handles the readiness probe
func healthHandler(server *Server, probename string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Must(logger.FromContext(r.Context())).Child("health", probename)

		if !server.IsReady() {
			log.Errorf("Webserver not ready yet")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if core.GetEnvAsBool("TRACE_PROBE", false) {
			log.Infof("The application is ready")
		}
		w.WriteHeader(http.StatusOK)
	})
}
