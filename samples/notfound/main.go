package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/gildas/go-core"
	"github.com/gildas/go-logger"
	"github.com/gildas/wess"
)

var APP = "sample-simple"

// notFoundHandler is used when the routes are not found
//
// This code is used to show how to overwrite the default handlers
func notFoundHandler(log *logger.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Must(logger.FromContext(r.Context(), log)).Child("simple", "notfound")

		log.Debugf("Request Headers: %#+v", r.Header)
		log.Errorf("Route not found: %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("404 Not Found"))
	})
}

func main() {
	port := flag.Int("port", core.GetEnvAsInt("PORT", 80), "The port to listen on")
	flag.Parse()

	log := logger.Create(APP)
	defer log.Flush()
	log.Infof(strings.Repeat("-", 80))
	log.Infof("Starting %s v%s (%s)", APP, wess.VERSION, runtime.GOARCH)
	log.Infof("Log Destination: %s", log)

	if *port == 0 {
		log.Fatalf("No port specified")
		os.Exit(-1)
	}

	server := wess.NewServer(wess.ServerOptions{
		Port:            *port,
		Logger:          log,
		NotFoundHandler: notFoundHandler(log),
	})

	server.AddRouteWithFunc("GET", "/hello", func(w http.ResponseWriter, r *http.Request) {
		log := logger.Must(logger.FromContext(r.Context())).Child(nil, "hello")

		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "text/plain")
		written, _ := w.Write([]byte("Hello, World!"))
		log.Debugf("Witten %d bytes", written)
	})

	shutdown, _, _ := server.Start(context.Background())
	<-shutdown
}
