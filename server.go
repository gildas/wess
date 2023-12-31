package wess

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

// ServerOptions defines the options for the server
type ServerOptions struct {
	Address   string // The address to listen on, Default: all interfaces
	Port      int    // The port to listen on, Default: 80
	ProbePort int    // The port to listen on for the health probe, Default: 0 (disabled)

	// The gorilla/mux router to use.
	// If not specified, a new one is created.
	Router *mux.Router

	// HealthRootPath is the root path for the health probe.
	// By default: "/healthz"
	HealthRootPath string

	// DisableGeneralOptionsHandler, if true, passes "OPTIONS *"
	// requests to the Handler, otherwise responds with 200 OK
	// and Content-Length: 0.
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
	// If not specified, nothing gets logged.
	Logger *logger.Logger

	// ErrorLog specifies an optional logger for errors accepting
	// connections, unexpected behavior from handlers, and
	// underlying FileSystem errors.
	// If nil, logging is done via the Logger defined above.
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

	// Configurable Handler to be used when the request method
	// does not match the route.
	MethodNotAllowedHandler http.Handler

	// AllowedCORSHeaders is the list of allowed headers
	AllowedCORSHeaders []string

	// AllowedCORSMethods is the list of allowed methods
	AllowedCORSMethods []string

	// AllowedCORSOrigins is the list of allowed origins
	AllowedCORSOrigins []string

	// ExposedCORSHeader is the list of headers that are safe to expose to
	// the API of a CORS API specification
	ExposedCORSHeaders []string

	// CORSMaxAge indicates how long (in seconds) the results of a preflight
	// request can be cached
	CORSMaxAge time.Duration

	// CORSAllowCredentials indicates whether the request can include user credentials
	// like cookies, HTTP authentication or client side SSL certificates
	CORSAllowCredentials bool

	// AllowOriginFunc is a custom function to validate the origin
	AllowOriginFunc func(origin string) bool

	// CPRSAllowPrivateNetwork indicates whether to acceptrequests
	// over a private network
	CORSAllowPrivateNetwork bool

	// CORSOptionsPasthrough instructs preflight to let other potential next handlers to
	// process the OPTIONS method. Turn this on if your application handles OPTIONS.
	CORSOptionsPasthrough bool

	// CORSOptionsSuccessStatus provides a status code to use for
	// successful OPTIONS requests, instead of http.StatusNoContent (204)
	CORSOptionsSuccessStatus int
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

	var probelogger *logger.Logger
	if options.Logger == nil {
		options.Logger = logger.Create("wess", &logger.NilStream{})
		probelogger = options.Logger
	} else {
		options.Logger = options.Logger.Child("webserver", "webserver")
		if core.GetEnvAsBool("TRACE_PROBE", false) {
			probelogger = options.Logger.Child("probeserver", "probeserver")
		} else {
			probelogger = logger.Create("wess", &logger.NilStream{})
		}
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
		options.Router.NotFoundHandler = notFoundHandler(options.Logger)
	}
	if options.MethodNotAllowedHandler != nil {
		options.Router.MethodNotAllowedHandler = options.MethodNotAllowedHandler
	} else {
		options.Router.MethodNotAllowedHandler = methodNotAllowedHandler(options.Logger)
	}

	var probeserver *http.Server
	var proberouter *mux.Router

	if options.ProbePort > 0 {
		if options.HealthRootPath == "" {
			options.HealthRootPath = "/healthz"
		}
		if options.ProbePort == options.Port {
			proberouter = options.Router.PathPrefix(options.HealthRootPath).Subrouter()
			proberouter.Use(probelogger.HttpHandler())
		} else {
			router := mux.NewRouter().StrictSlash(true)
			router.Use(probelogger.HttpHandler())
			proberouter = router.PathPrefix(options.HealthRootPath).Subrouter()
			proberouter.MethodNotAllowedHandler = methodNotAllowedHandler(probelogger)
			proberouter.NotFoundHandler = notFoundHandler(probelogger)
			probeserver = &http.Server{
				Addr:              fmt.Sprintf("%s:%d", options.Address, options.ProbePort),
				Handler:           router,
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
	}

	var webhandler http.Handler

	if len(options.AllowedCORSMethods) > 0 || len(options.AllowedCORSHeaders) > 0 || len(options.AllowedCORSOrigins) > 0 {
		options.Logger.Infof("CORS is enabled on the webserver")
		if len(options.AllowedCORSMethods) > 0 {
			options.Logger.Debugf("CORS: Allowed Methods: %s", strings.Join(options.AllowedCORSMethods, ", "))
		}
		if len(options.AllowedCORSHeaders) > 0 {
			options.Logger.Debugf("CORS: Allowed Headers: %s", strings.Join(options.AllowedCORSHeaders, ", "))
		}
		if len(options.AllowedCORSOrigins) > 0 {
			options.Logger.Debugf("CORS: Allowed Origins: %s", strings.Join(options.AllowedCORSOrigins, ", "))
		}
		if len(options.ExposedCORSHeaders) > 0 {
			options.Logger.Debugf("CORS: Exposed Headers: %s", strings.Join(options.ExposedCORSHeaders, ", "))
		}
		if options.CORSMaxAge > 0 {
			options.Logger.Debugf("CORS: Max Age: %s", options.CORSMaxAge)
		}
		options.Logger.Debugf("CORS: Allow Credentials: %t", options.CORSAllowCredentials)
		options.Logger.Debugf("CORS: Allow Private Network: %t", options.CORSAllowPrivateNetwork)
		options.Logger.Debugf("CORS: Options Passthrough: %t", options.CORSOptionsPasthrough)
		if options.CORSOptionsSuccessStatus > 0 {
			options.Logger.Debugf("CORS: Options Success Status: %d", options.CORSOptionsSuccessStatus)
		}
		corsMiddleware := cors.New(cors.Options{
			AllowedOrigins:       options.AllowedCORSOrigins,
			AllowedHeaders:       options.AllowedCORSHeaders,
			AllowedMethods:       options.AllowedCORSMethods,
			ExposedHeaders:       options.ExposedCORSHeaders,
			MaxAge:               int(options.CORSMaxAge.Seconds()),
			AllowOriginFunc:      options.AllowOriginFunc,
			AllowCredentials:     options.CORSAllowCredentials,
			AllowPrivateNetwork:  options.CORSAllowPrivateNetwork,
			OptionsPassthrough:   options.CORSOptionsPasthrough,
			OptionsSuccessStatus: options.CORSOptionsSuccessStatus,
			Debug:                options.Logger.ShouldWrite(logger.DEBUG, "cors", "cors"),
		})
		corsMiddleware.Log = CorsLogger{options.Logger.Child("cors", "cors")}
		webhandler = corsMiddleware.Handler(options.Router)
	} else {
		webhandler = options.Router
	}

	return &Server{
		ShutdownTimeout: options.ShutdownTimeout,
		logger:          options.Logger,
		webrouter:       options.Router,
		proberouter:     proberouter,
		probeserver:     probeserver,
		webserver: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", options.Address, options.Port),
			Handler:           webhandler,
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

// AddRoute adds a route to the server
func (server Server) AddRoute(method, path string, handler http.Handler) {
	server.webrouter.Methods(method).Path(path).Handler(handler)
}

// AddRouteWithFunc adds a route to the server
func (server Server) AddRouteWithFunc(method, path string, handlerFunc http.HandlerFunc) {
	server.webrouter.Methods(method).Path(path).HandlerFunc(handlerFunc)
}

// SubRouter creates a subrouter
func (server Server) SubRouter(path string) *mux.Router {
	return server.webrouter.PathPrefix(path).Subrouter()
}

// Start starts the server
//
// Callers should wait on the returned shutdown channel.
//
// Callers can stop the server programatically by sending a signal on the returned stop channel.
func (server *Server) Start(context context.Context) (shutdown chan error, stop chan os.Signal, err error) {
	log := server.getChildLogger(context, "webserver", "start")

	if server.proberouter != nil {
		server.healthRoutes(server.proberouter)
	}

	log.Infof("Listening on %s", server.webserver.Addr)
	server.logRoutes(log.ToContext(context), server.webrouter)

	if server.probeserver != nil {
		plog := log.Child("probeserver", nil)
		plog.Infof("Health probes listening on %s", server.probeserver.Addr)
		server.logRoutes(plog.ToContext(context), server.probeserver.Handler.(*mux.Router))
		if err = server.waitForStart(plog.ToContext(context), server.probeserver); err != nil {
			return nil, nil, err
		}
	}

	if err = server.waitForStart(log.ToContext(context), server.webserver); err != nil {
		return nil, nil, err
	}
	shutdown, stop = server.waitForShutdown(log.ToContext(context))
	return
}

// logRoutes logs the routes
func (server Server) logRoutes(context context.Context, router *mux.Router) {
	log := server.getChildLogger(context, nil, "routes")
	log.Infof("Serving routes:")
	_ = router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
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
				started <- err
			}
		}
	}(started)

	select {
	case err := <-started:
		if err != nil {
			return errors.RuntimeError.Wrap(err)
		}
	case <-time.After(time.Second * 1):
		if httpserver == server.probeserver {
			log.Child("probeserver", "start").Infof("Health probe server started")
		} else {
			log.Infof("WEB Server started")
		}
	}
	return nil
}

// waitForShutdown waits for the server to shutdown
func (server Server) waitForShutdown(ctx context.Context) (shutdown chan error, stop chan os.Signal) {
	stop = make(chan os.Signal, 1)
	shutdown = make(chan error, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log := server.getChildLogger(ctx, "webserver", "shutdown")
		sig := <-stop
		context, cancel := context.WithTimeout(ctx, server.ShutdownTimeout)
		defer cancel()
		log.Infof("Received signal %s, shutting down...", sig)

		atomic.StoreInt32(&server.healthStatus, 0)

		// Stopping the probe server
		if server.probeserver != nil {
			plog := log.Child("probeserver", "shutdown")

			plog.Debugf("Stopping the probe server")
			server.probeserver.SetKeepAlivesEnabled(false)
			if err := server.probeserver.Shutdown(context); err != nil {
				plog.Errorf("Failed to gracefully shutdown the probe server", errors.RuntimeError.Wrap(err))
				_ = server.probeserver.Close()
			} else {
				plog.Infof("Probe Server stopped")
			}
		}

		// Stopping the WEB server
		log.Debugf("Stopping the WEB server")
		server.webserver.SetKeepAlivesEnabled(false)
		if err := server.webserver.Shutdown(context); err != nil {
			err = errors.RuntimeError.Wrap(err)
			log.Errorf("Failed to gracefully shutdown the server", err)
			_ = server.webserver.Close()
			shutdown <- err
		} else {
			log.Infof("WEB Server stopped")
		}
		shutdown <- nil
	}()
	return shutdown, stop
}

// getChildLogger gets a child logger
func (server Server) getChildLogger(context context.Context, topic, scope interface{}, params ...interface{}) *logger.Logger {
	return logger.Must(logger.FromContext(context, server.logger)).Child(topic, scope, params...)
}
