package wess

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gildas/go-core"
	"github.com/gildas/go-logger"
	"github.com/gorilla/websocket"
)

// WebSocketHandler is a function that will handle a WebSocket connection
type WebSocketHandler func(w http.ResponseWriter, r *http.Request, conn *websocket.Conn)

// WebSocketHandlerFunc is a function that will handle a WebSocket connection
type WebSocketHandlerFunc func(w http.ResponseWriter, r *http.Request, conn *websocket.Conn)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		log := logger.Must(logger.FromContext(r.Context())).Child("websocket", "checkorigin")
		origin := r.Header.Get("Origin")

		if len(origin) == 0 {
			log.Debugf("No Origin Header, accepting...")
			return true
		}

		originURL, err := url.Parse(origin)
		if err != nil {
			log.Errorf("Failed to parse the Origin Header: %s", origin, err)
			return false
		}

		allowedOrigins := strings.Split(core.GetEnvAsString("WEBSOCKET_ALLOWED_ORIGINS", ""), ",")
		if len(allowedOrigins) == 0 && originURL.Host != r.Host {
			log.Errorf("Origin %s is not allowed as it differs from %s", origin, r.Host)
			return false
		}

		for _, allowedOrigin := range allowedOrigins {
			allowedOrigin = strings.TrimSpace(allowedOrigin)
			if allowedOrigin == "*" || allowedOrigin == origin {
				log.Infof("Origin %s is allowed", origin)
				return true
			}
		}
		log.Errorf("Origin %s is not allowed as it does not belong to: ", origin, strings.Join(allowedOrigins, ", "))
		return false
	},
}

// ServeHTTP serves the WebSocket
func (handler WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, responseHeader http.Header) {
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		core.RespondWithError(w, http.StatusBadRequest, err)
		return
	}
	handler(w, r, conn)
}

func (server *Server) AddWebSocketRouteWithHandlerFunc(path string, handler WebSocketHandlerFunc) {
	server.AddRouteWithFunc(http.MethodGet, path, func(w http.ResponseWriter, r *http.Request) {
		handler.ServeWebSocket(w, r, nil)
	})
}
