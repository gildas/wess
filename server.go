package wess

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gildas/go-core"
	"github.com/gildas/go-logger"
	"github.com/gorilla/mux"
)

// ServerOptions defines the options for the server
type ServerOptions struct {
	Address   string      // The address to listen on, Default: all interfaces
	Port      int         // The port to listen on, Default: 80
	ProbePort int         // The port to listen on for the health probe, Default: 0 (disabled)
	Router    *mux.Router // The router to use

	// HealthRootPath is the root path for the health probe, Default: "/healthz"
	HealthRootPath string

	// DisableGeneralOptionsHandler, if true, passes "OPTIONS *" requests to the Handler,
	// otherwise responds with 200 OK and Content-Length: 0.
	DisableGeneralOptionsHandler bool

	// TLSConfig optionally provides a TLS configuration for use
	// by ServeTLS and ListenAndServeTLS. Note that this value is
	// cloned by ServeTLS and ListenAndServeTLS, so it's not
	// possible to modify the configuration with methods like
	// tls.Config.SetSessionTicketKeys. To use
	// SetSessionTicketKeys, use Server.Serve with a TLS Listener
	// instead.
	TLSConfig *tls.Config

	// ReadTimeout is the maximum duration for reading the entire
	// request, including the body. A zero or negative value means
	// there will be no timeout.
	//
	// Because ReadTimeout does not let Handlers make per-request
	// decisions on each request body's acceptable deadline or
	// upload rate, most users will prefer to use
	// ReadHeaderTimeout. It is valid to use them both.
	ReadTimeout time.Duration

	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers. The connection's read deadline is reset
	// after reading the headers and the Handler can decide what
	// is considered too slow for the body. If ReadHeaderTimeout
	// is zero, the value of ReadTimeout is used. If both are
	// zero, there is no timeout.
	ReadHeaderTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out
	// writes of the response. It is reset whenever a new
	// request's header is read. Like ReadTimeout, it does not
	// let Handlers make decisions on a per-request basis.
	// A zero or negative value means there will be no timeout.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the
	// next request when keep-alives are enabled. If IdleTimeout
	// is zero, the value of ReadTimeout is used. If both are
	// zero, there is no timeout.
	IdleTimeout time.Duration

	// ShutdownTimeout is the maximum amount of time to wait for the
	// server to shutdown. Default: 15 seconds
	ShutdownTimeout time.Duration

	// MaxHeaderBytes controls the maximum number of bytes the
	// server will read parsing the request header's keys and
	// values, including the request line. It does not limit the
	// size of the request body.
	// If zero, DefaultMaxHeaderBytes is used.
	MaxHeaderBytes int

	// TLSNextProto optionally specifies a function to take over
	// ownership of the provided TLS connection when an ALPN
	// protocol upgrade has occurred. The map key is the protocol
	// name negotiated. The Handler argument should be used to
	// handle HTTP requests and will initialize the Request's TLS
	// and RemoteAddr if not already set. The connection is
	// automatically closed when the function returns.
	// If TLSNextProto is not nil, HTTP/2 support is not enabled
	// automatically.
	TLSNextProto map[string]func(*http.Server, *tls.Conn, http.Handler)

	// ConnState specifies an optional callback function that is
	// called when a client connection changes state. See the
	// ConnState type and associated constants for details.
	ConnState func(net.Conn, http.ConnState)

	// Logger is the logger used by this server
	Logger *logger.Logger

	// ErrorLog specifies an optional logger for errors accepting
	// connections, unexpected behavior from handlers, and
	// underlying FileSystem errors.
	// If nil, logging is done via the log package's standard logger.
	ErrorLog *log.Logger

	// BaseContext optionally specifies a function that returns
	// the base context for incoming requests on this server.
	// The provided Listener is the specific Listener that's
	// about to start accepting requests.
	// If BaseContext is nil, the default is context.Background().
	// If non-nil, it must return a non-nil context.
	BaseContext func(net.Listener) context.Context

	// ConnContext optionally specifies a function that modifies
	// the context used for a new connection c. The provided ctx
	// is derived from the base context and has a ServerContextKey
	// value.
	ConnContext func(ctx context.Context, c net.Conn) context.Context

	// Configurable Handler to be used when no route matches.
	NotFoundHandler http.Handler

	// Configurable Handler to be used when the request method does not match the route.
	MethodNotAllowedHandler http.Handler
}

// Server defines a Web Server
type Server struct {
	// ShutdownTimeout is the maximum amount of time to wait for the
	// server to shutdown. Default: 15 seconds
	ShutdownTimeout time.Duration

	healthStatus int32 // 0: Not Ready, 1: Ready
	webrouter    *mux.Router
	webserver    *http.Server
	proberouter  *mux.Router
	probeserver  *http.Server
	logger       *logger.Logger
}

// NewServer creates a new Web Server
func NewServer(options ServerOptions) *Server {
	if options.Port == 0 {
		options.Port = 80
	}
	if options.Address == "" {
		options.Address = "0.0.0.0"
	}
	if options.ShutdownTimeout == 0 {
		options.ShutdownTimeout = time.Second * 15
	}
	if options.Logger == nil {
		options.Logger = logger.Create("wess", &logger.NilStream{}).Child("webserver", "webserver")
	} else {
		options.Logger = options.Logger.Child("webserver", "webserver")
	}
	if options.ErrorLog == nil {
		options.ErrorLog = options.Logger.AsStandardLog()
	}
	if options.Router == nil {
		options.Router = mux.NewRouter().StrictSlash(true)
	}
	options.Router.Use(options.Logger.HttpHandler())

	if options.NotFoundHandler != nil {
		options.Router.NotFoundHandler = options.NotFoundHandler
	} else {
		options.Router.NotFoundHandler = notFoundHandler()
	}
	if options.MethodNotAllowedHandler != nil {
		options.Router.MethodNotAllowedHandler = options.MethodNotAllowedHandler
	} else {
		options.Router.MethodNotAllowedHandler = methodNotAllowedHandler()
	}

	var probeserver *http.Server
	var proberouter *mux.Router

	if options.ProbePort > 0 {
		if options.HealthRootPath == "" {
			options.HealthRootPath = "/healthz"
		}
		if options.ProbePort == options.Port {
			proberouter = options.Router.PathPrefix(options.HealthRootPath).Subrouter()
		} else {
			proberouter = mux.NewRouter().StrictSlash(true).PathPrefix(options.HealthRootPath).Subrouter()
			proberouter.NotFoundHandler = notFoundHandler()
			proberouter.MethodNotAllowedHandler = methodNotAllowedHandler()
			probeserver = &http.Server{
				Addr:              fmt.Sprintf("%s:%d", options.Address, options.ProbePort),
				Handler:           proberouter,
				TLSConfig:         options.TLSConfig,
				ReadTimeout:       options.ReadTimeout,
				ReadHeaderTimeout: options.ReadHeaderTimeout,
				WriteTimeout:      options.WriteTimeout,
				IdleTimeout:       options.IdleTimeout,
				MaxHeaderBytes:    options.MaxHeaderBytes,
				TLSNextProto:      options.TLSNextProto,
				ConnState:         options.ConnState,
				ErrorLog:          options.ErrorLog,
				BaseContext:       options.BaseContext,
				ConnContext:       options.ConnContext,
			}
		}
		if len(core.GetEnvAsString("TRACE_PROBE", "")) > 0 {
			proberouter.Use(options.Logger.HttpHandler())
		} else {
			proberouter.Use(logger.Create("wess", &logger.NilStream{}).HttpHandler())
		}
	}

	return &Server{
		ShutdownTimeout: options.ShutdownTimeout,
		logger:          options.Logger,
		webrouter:       options.Router,
		proberouter:     proberouter,
		probeserver:     probeserver,
		webserver: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", options.Address, options.Port),
			Handler:           options.Router,
			TLSConfig:         options.TLSConfig,
			ReadTimeout:       options.ReadTimeout,
			ReadHeaderTimeout: options.ReadHeaderTimeout,
			WriteTimeout:      options.WriteTimeout,
			IdleTimeout:       options.IdleTimeout,
			MaxHeaderBytes:    options.MaxHeaderBytes,
			TLSNextProto:      options.TLSNextProto,
			ConnState:         options.ConnState,
			ErrorLog:          options.ErrorLog,
			BaseContext:       options.BaseContext,
			ConnContext:       options.ConnContext,
		},
	}
}

// IsReady tells if the server is ready
func (server Server) IsReady() bool {
	return atomic.LoadInt32(&server.healthStatus) == 1
}

// Start starts the server
func (server Server) Start(context context.Context) (shutdown chan error, err error) {
	log := server.getChildLogger(context, "webserver", "start")

	log.Infof("Listening on %s", server.webserver.Addr)
	server.logRoutes(log.ToContext(context))

	if server.proberouter != nil {
		server.healthRoutes(server.proberouter)
	}
	if server.probeserver != nil {
		log.Infof("Health probe routes will be served on port %s", server.probeserver.Addr)
		if err = server.waitForStart(log.ToContext(context), server.probeserver); err != nil {
			return nil, err
		}
	}

	if err = server.waitForStart(log.ToContext(context), server.webserver); err != nil {
		return nil, err
	}
	return server.waitForShutdown(log.ToContext(context)), nil
}

// logRoutes logs the routes
func (server Server) logRoutes(context context.Context) {
	log := server.getChildLogger(context, "webserver", "routes")
	log.Infof("Serving routes:")
	_ = server.webrouter.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		message := strings.Builder{}
		args := []interface{}{}

		if methods, err := route.GetMethods(); err == nil {
			message.WriteString("%s ")
			args = append(args, strings.Join(methods, ", "))
		}
		if path, err := route.GetPathTemplate(); err == nil {
			message.WriteString("%s ")
			args = append(args, path)
		}
		if path, err := route.GetPathRegexp(); err == nil {
			message.WriteString("%s ")
			args = append(args, path)
		}
		log.Infof(message.String(), args...)
		return nil
	})
}

// waitForStart waits for the server to start
func (server *Server) waitForStart(context context.Context, httpserver *http.Server) error {
	log := server.getChildLogger(context, "webserver", "start")
	started := make(chan error)

	go func(started chan error) {
		atomic.StoreInt32(&server.healthStatus, 1)
		// In case of success, this func never returns
		if err := httpserver.ListenAndServe(); err != nil {
			atomic.StoreInt32(&server.healthStatus, 0)
			if err.Error() != "http: Server closed" {
				log.Fatalf("Failed to start the WEB server on port %d", server.webserver.Addr, err)
				started <- err
			}
		}
	}(started)

	select {
	case err := <-started:
		if err != nil {
			return err
		}
	case <-time.After(time.Second * 1):
		log.Infof("WEB Server started")
	}
	return nil
}

// waitForShutdown waits for the server to shutdown
func (server Server) waitForShutdown(ctx context.Context) (shutdown chan error) {
	interruptChannel := make(chan os.Signal, 1)
	shutdown = make(chan error, 1)

	go func() {
		log := server.getChildLogger(ctx, "webserver", "shutdown")
		sig := <-interruptChannel
		context, cancel := context.WithTimeout(ctx, server.ShutdownTimeout)
		defer cancel()
		log.Infof("Received signal %s, shutting down...", sig)

		// Stopping the probe server
		/*
			if *probePort > 0 && *probePort != *port {
				log.Debugf("Stopping the probe server")
				probeServer.SetKeepAlivesEnabled(false)
				if err := probeServer.Shutdown(context); err != nil {
					log.Errorf("Failed to gracefully shutdown the probe server: %s", err)
				} else {
					log.Infof("Probe Server stopped")
				}
			}
		*/

		// Stopping the WEB server
		log.Debugf("Stopping the WEB server")
		atomic.StoreInt32(&server.healthStatus, 0)
		server.webserver.SetKeepAlivesEnabled(false)
		if err := server.webserver.Shutdown(context); err != nil {
			log.Errorf("Failed to gracefully shutdown the server: %s", err)
			shutdown <- err
		} else {
			log.Infof("WEB Server stopped")
		}
		shutdown <- nil
	}()
	return shutdown
}

// getChildLogger gets a child logger
func (server Server) getChildLogger(context context.Context, topic, scope interface{}, params ...interface{}) *logger.Logger {
	return logger.Must(logger.FromContext(context, server.logger)).Child(topic, scope, params...)
}
