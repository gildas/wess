package main

import (
	"context"
	"embed"
	"flag"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/gildas/go-core"
	"github.com/gildas/go-logger"
	"github.com/gildas/wess"
)

var (
	APP = "sample-web"

	//go:embed all:frontend/dist
	frontendFS embed.FS
)

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
		Port:   *port,
		Logger: log,
	})

	server.AddRouteWithFunc("GET", "/fortune", func(w http.ResponseWriter, r *http.Request) {
		log := logger.Must(logger.FromContext(r.Context())).Child(nil, "fortune")
		log.Infof("Returning a fortune")
		w.Header().Set("Content-Type", "text/plain")
		written, _ := w.Write([]byte("You will be rich!"))
		log.Debugf("Wrote %d bytes", written)
	})

	apiRouter := server.SubRouter("/api")
	apiRouter.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.Must(logger.FromContext(r.Context())).Child("api", "middleware")
			log.Infof("This is a middleware") // Like an authentication middleware
			next.ServeHTTP(w, r)
		})
	})
	apiRouter.Methods("GET").Path("/me").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Must(logger.FromContext(r.Context())).Child("api", "me")

		log.Infof("Returning current user")
		w.Header().Set("Content-Type", "application/json")
		written, _ := w.Write([]byte(`{"name": "John Doe"}`))
		log.Debugf("Wrote %d bytes", written)
	})

	_ = server.AddFrontend("/", frontendFS, "frontend/dist")

	shutdown, _ := server.Start(context.Background())
	<-shutdown
}
