package wess

import (
	"net/http"

	"github.com/gildas/go-logger"
)

// notFoundHandler is the handler for the 404 Not Found
func notFoundHandler(log *logger.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Must(logger.FromContext(r.Context(), log)).Child(nil, "notfound")

		log.Debugf("Request Headers: %#+v", r.Header)
		log.Errorf("Route not found: %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("404 Not Found"))
	})
}
