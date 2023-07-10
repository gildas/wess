package wess

import (
	"net/http"

	"github.com/gildas/go-logger"
)

// methodNotAllowedHandler is the handler for the 405 Method Not Allowed
func methodNotAllowedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Must(logger.FromContext(r.Context())).Child(nil, "notfound")

		log.Debugf("Request Headers: %#+v", r.Header)
		log.Errorf("Method Not Allowed: %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("405 Method Not Allowed"))
	})
}
