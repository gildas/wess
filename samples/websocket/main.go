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
	"github.com/gorilla/websocket"
)

var APP = "websocket"

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

	server.AddWebSocketRouteWithHandlerFunc("/ws", func(w http.ResponseWriter, r *http.Request, conn *websocket.Conn) {
		log := logger.Must(logger.FromContext(r.Context())).Child(nil, "ws")

		log.Infof("WebSocket connection from %s", conn.RemoteAddr())
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Hello, World!"))
		conn.Close()
	})

	router := server.SubRouter("/api")
	router.Path("/ws1").Handler(wess.WebSocketHandler(func(w http.ResponseWriter, r *http.Request, conn *websocket.Conn) {
		log := logger.Must(logger.FromContext(r.Context())).Child(nil, "ws")

		log.Infof("WebSocket connection from %s", conn.RemoteAddr())
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Hello, World!"))
		conn.Close()
	}))

	router.Path("/ws2").HandlerFunc(wess.WebSocketHandlerFunc(func(w http.ResponseWriter, r *http.Request, conn *websocket.Conn) {
		log := logger.Must(logger.FromContext(r.Context())).Child(nil, "ws")

		log.Infof("WebSocket connection from %s", conn.RemoteAddr())
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Hello, World!"))
		conn.Close()
	}))

	shutdown, _, _ := server.Start(context.Background())
	<-shutdown
}
