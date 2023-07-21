package main

import (
	"context"
	"embed"
	"flag"
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

	_ = server.AddFrontend("/", frontendFS, "frontend/dist")
	shutdown, _, _ := server.Start(context.Background())
	<-shutdown
}
