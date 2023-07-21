package wess

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/gildas/go-request"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/suite"
)

type ServerSuite struct {
	suite.Suite
	Name   string
	Logger *logger.Logger
	Start  time.Time
}

var (
	//go:embed all:testdata
	frontendFS embed.FS
)

func TestServerSuite(t *testing.T) {
	suite.Run(t, new(ServerSuite))
}

// *****************************************************************************
// Suite Tools

func (suite *ServerSuite) SetupSuite() {
	_ = godotenv.Load()
	suite.Name = strings.TrimSuffix(reflect.TypeOf(suite).Elem().Name(), "Suite")
	suite.Logger = logger.Create("test",
		&logger.FileStream{
			Path:         fmt.Sprintf("./log/test-%s.log", strings.ToLower(suite.Name)),
			Unbuffered:   true,
			SourceInfo:   true,
			FilterLevels: logger.NewLevelSet(logger.TRACE),
		},
	).Child("test", "test")
	suite.Logger.Infof("Suite Start: %s %s", suite.Name, strings.Repeat("=", 80-14-len(suite.Name)))

	err := os.MkdirAll("./tmp", 0755)
	suite.Require().Nilf(err, "Failed creating tmp directory, err=%+v", err)
}

func (suite *ServerSuite) TearDownSuite() {
	suite.Logger.Debugf("Tearing down")
	if suite.T().Failed() {
		suite.Logger.Warnf("At least one test failed, we are not cleaning")
		suite.T().Log("At least one test failed, we are not cleaning")
	} else {
		suite.Logger.Infof("All tests succeeded, we are cleaning")
	}
	suite.Logger.Infof("Suite End: %s %s", suite.Name, strings.Repeat("=", 80-12-len(suite.Name)))
}

func (suite *ServerSuite) BeforeTest(suiteName, testName string) {
	suite.Logger.Infof("Test Start: %s %s", testName, strings.Repeat("-", 80-13-len(testName)))
	suite.Start = time.Now()
}

func (suite *ServerSuite) AfterTest(suiteName, testName string) {
	duration := time.Since(suite.Start)
	if suite.T().Failed() {
		suite.Logger.Errorf("Test %s failed", testName)
	}
	suite.Logger.Record("duration", duration.String()).Infof("Test End: %s %s", testName, strings.Repeat("-", 80-11-len(testName)))
}

func FuncAddress(f any) uintptr {
	return reflect.ValueOf(f).Pointer()
}

// *****************************************************************************

func (suite *ServerSuite) TestCanInitializeWithDefauts() {
	server := NewServer(ServerOptions{})
	suite.Require().NotNil(server, "Server should not be nil")
	suite.Assert().Equal(15*time.Second, server.ShutdownTimeout)
	suite.Assert().Nil(server.proberouter, "Server should not have a Probe Router")
	suite.Assert().Nil(server.probeserver, "Server should not have a Probe Server")
	suite.Require().NotNil(server.logger, "Server should have a Logger")
	suite.Require().NotNil(server.webrouter, "Server should have a Web Router")
	suite.Assert().Equal(FuncAddress(notFoundHandler(server.logger)), FuncAddress(server.webrouter.NotFoundHandler), "Server should use the default NotFound Handler")
	suite.Assert().Equal(FuncAddress(methodNotAllowedHandler(server.logger)), FuncAddress(server.webrouter.MethodNotAllowedHandler), "Server should use the default NotFound Handler")
	suite.Require().NotNil(server.webserver, "Server should have a Web Server")
	suite.Assert().Equal("0.0.0.0:80", server.webserver.Addr)
	suite.Assert().NotNil(server.webserver.ErrorLog, "Server should have an Error Log")
	suite.Assert().False(server.IsReady())
}

func (suite *ServerSuite) TestCanInitializeWithLogger() {
	server := NewServer(ServerOptions{
		Logger: suite.Logger,
	})
	suite.Require().NotNil(server, "Server should not be nil")
}

func (suite *ServerSuite) TestCanInitializeWithErrorHandlers() {
	localNotFoundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	localMethodNotAllowed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	server := NewServer(ServerOptions{
		NotFoundHandler:         localNotFoundHandler,
		MethodNotAllowedHandler: localMethodNotAllowed,
	})
	suite.Require().NotNil(server, "Server should not be nil")
	suite.Assert().Equal(FuncAddress(localNotFoundHandler), FuncAddress(server.webrouter.NotFoundHandler), "Server should use the provided NotFound Handler")
	suite.Assert().Equal(FuncAddress(localMethodNotAllowed), FuncAddress(server.webrouter.MethodNotAllowedHandler), "Server should use the provided NotFound Handler")
}

func (suite *ServerSuite) TestCanInitializeWithProbePort() {
	server := NewServer(ServerOptions{
		ProbePort:      8000,
		HealthRootPath: "/health",
	})
	suite.Require().NotNil(server, "Server should not be nil")
	suite.Assert().NotNil(server.proberouter, "Server should have a Probe Router")
	suite.Assert().NotNil(server.probeserver, "Server should have a Probe Server")
	suite.Assert().Equal("0.0.0.0:80", server.webserver.Addr)
	suite.Assert().Equal("0.0.0.0:8000", server.probeserver.Addr)
	// test the router path

	server = NewServer(ServerOptions{
		ProbePort: 80,
	})
	suite.Require().NotNil(server, "Server should not be nil")
	suite.Assert().NotNil(server.proberouter, "Server should have a Probe Router")
	suite.Assert().Nil(server.probeserver, "Server should not have a Probe Server")
	suite.Assert().Equal("0.0.0.0:80", server.webserver.Addr)

	traceenv := os.Getenv("TRACE_PROBE")
	defer os.Setenv("TRACE_PROBE", traceenv)
	os.Setenv("TRACE_PROBE", "true")
	server = NewServer(ServerOptions{
		ProbePort: 80,
		Logger:    suite.Logger,
	})
	suite.Require().NotNil(server, "Server should not be nil")
	suite.Assert().NotNil(server.proberouter, "Server should have a Probe Router")
	suite.Assert().Nil(server.probeserver, "Server should not have a Probe Server")
	suite.Assert().Equal("0.0.0.0:80", server.webserver.Addr)
}

func (suite *ServerSuite) TestCanInitializeWithCORS() {
	server := NewServer(ServerOptions{
		AllowedCORSMethods:       []string{"GET", "POST"},
		AllowedCORSHeaders:       []string{"X-Test"},
		AllowedCORSOrigins:       []string{"http://localhost:8080"},
		ExposedCORSHeaders:       []string{"X-Test"},
		CORSMaxAge:               3600 * time.Second,
		CORSOptionsSuccessStatus: 200,
	})
	suite.Require().NotNil(server, "Server should not be nil")
}

func (suite *ServerSuite) TestCanAddRoutesWithHandler() {
	server := NewServer(ServerOptions{})
	suite.Require().NotNil(server, "Server should not be nil")
	server.AddRoute(http.MethodGet, "/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	err := server.webrouter.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		methods, err := route.GetMethods()
		suite.Require().NoError(err, "Failed getting the methods")
		template, err := route.GetPathTemplate()
		suite.Require().NoError(err, "Failed getting the template")
		suite.Logger.Debugf("Route: method=%s, template=%s", methods, template)
		suite.Assert().Equal([]string{http.MethodGet}, methods)
		suite.Assert().Equal("/test", template)
		return nil
	})
	suite.Require().NoError(err, "Failed walking the routes")
}

func (suite *ServerSuite) TestCanAddRoutesWithHandlerFunc() {
	server := NewServer(ServerOptions{})
	suite.Require().NotNil(server, "Server should not be nil")
	server.AddRouteWithFunc(http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	err := server.webrouter.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		methods, err := route.GetMethods()
		suite.Require().NoError(err, "Failed getting the methods")
		template, err := route.GetPathTemplate()
		suite.Require().NoError(err, "Failed getting the template")
		suite.Logger.Debugf("Route: method=%s, template=%s", methods, template)
		suite.Assert().Equal([]string{http.MethodGet}, methods)
		suite.Assert().Equal("/test", template)
		return nil
	})
	suite.Require().NoError(err, "Failed walking the routes")
}

func (suite *ServerSuite) TestCanGetSubRouter() {
	server := NewServer(ServerOptions{})
	suite.Require().NotNil(server, "Server should not be nil")
	subrouter := server.SubRouter("/test")
	suite.Require().NotNil(subrouter, "SubRouter should not be nil")
}

func (suite *ServerSuite) TestCanAddFrontend() {
	server := NewServer(ServerOptions{})
	suite.Require().NotNil(server, "Server should not be nil")
	err := server.AddFrontend("/", frontendFS, "testdata")
	suite.Require().NoError(err, "Failed adding the frontend")

	err = server.AddFrontend("/", frontendFS, "/")
	suite.Require().Error(err, "Should have failed adding the frontend with wrong path")
}

func (suite *ServerSuite) TestCanStartAndShutdown() {
	server := NewServer(ServerOptions{
		Port:   9898,
		Logger: suite.Logger,
	})
	suite.Require().NotNil(server, "Server should not be nil")
	server.AddRouteWithFunc(http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	shutdown, stop, err := server.Start(context.Background())
	suite.Require().NoError(err, "Failed starting the server")

	go func() {
		time.Sleep(100 * time.Millisecond)
		suite.Assert().True(server.IsReady(), "Server should be ready")
		_, err = request.Send(&request.Options{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Host: "localhost:9898", Path: "/test"},
		}, nil)
		suite.Require().NoError(err, "Failed sending a /test request")
		_, err = request.Send(&request.Options{
			Method: http.MethodDelete,
			URL:    &url.URL{Scheme: "http", Host: "localhost:9898", Path: "/test"},
		}, nil)
		suite.Require().Error(err, "Should have failed sending a DELETE /test request")
		suite.Assert().ErrorIs(err, errors.HTTPMethodNotAllowed, "Error should have been a HTTPMethodNotAllowed but was %T", err)
		_, err = request.Send(&request.Options{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Host: "localhost:9898", Path: "/nowhere"},
		}, nil)
		suite.Require().Error(err, "Should have failed sending a /nowhere request")
		suite.Assert().ErrorIs(err, errors.HTTPNotFound, "Error should have been a HTTPNotFound but was %T", err)
		time.Sleep(400 * time.Millisecond)
		stop <- os.Interrupt
	}()

	err = <-shutdown
	suite.Require().NoError(err, "Failed shutting down the server")
	suite.Assert().False(server.IsReady(), "Server should not be ready anymore")
}

func (suite *ServerSuite) TestCanStartAndShutdownWithCORS() {
	server := NewServer(ServerOptions{
		Port:               9898,
		AllowedCORSMethods: []string{"GET", "POST"},
		Logger:             suite.Logger,
	})
	suite.Require().NotNil(server, "Server should not be nil")
	server.AddRouteWithFunc(http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	shutdown, stop, err := server.Start(context.Background())
	suite.Require().NoError(err, "Failed starting the server")

	go func() {
		time.Sleep(100 * time.Millisecond)
		suite.Assert().True(server.IsReady(), "Server should be ready")
		_, err = request.Send(&request.Options{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Host: "localhost:9898", Path: "/test"},
		}, nil)
		suite.Require().NoError(err, "Failed sending a /test request")
		_, err = request.Send(&request.Options{
			Method: http.MethodDelete,
			URL:    &url.URL{Scheme: "http", Host: "localhost:9898", Path: "/test"},
		}, nil)
		suite.Require().Error(err, "Should have failed sending a DELETE /test request")
		suite.Assert().ErrorIs(err, errors.HTTPMethodNotAllowed, "Error should have been a HTTPMethodNotAllowed but was %T", err)
		_, err = request.Send(&request.Options{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Host: "localhost:9898", Path: "/nowhere"},
		}, nil)
		suite.Require().Error(err, "Should have failed sending a /nowhere request")
		suite.Assert().ErrorIs(err, errors.HTTPNotFound, "Error should have been a HTTPNotFound but was %T", err)
		time.Sleep(400 * time.Millisecond)
		stop <- os.Interrupt
	}()

	err = <-shutdown
	suite.Require().NoError(err, "Failed shutting down the server")
	suite.Assert().False(server.IsReady(), "Server should not be ready anymore")
}

func (suite *ServerSuite) TestCanStartAndShutdownWithFrontend() {
	server := NewServer(ServerOptions{
		Port:   9898,
		Logger: suite.Logger,
	})
	suite.Require().NotNil(server, "Server should not be nil")
	err := server.AddFrontend("/", frontendFS, "testdata/frontend-good")
	suite.Require().NoError(err, "Failed adding the frontend")
	shutdown, stop, err := server.Start(context.Background())
	suite.Require().NoError(err, "Failed starting the server")

	go func() {
		time.Sleep(100 * time.Millisecond)
		suite.Assert().True(server.IsReady(), "Server should be ready")
		_, err = request.Send(&request.Options{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Host: "localhost:9898", Path: "/"},
		}, nil)
		suite.Require().NoError(err, "Failed sending a / request")
		_, err = request.Send(&request.Options{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Host: "localhost:9898", Path: "/private"},
		}, nil)
		suite.Require().Error(err, "Should have failed sending a /private request")
		suite.Assert().ErrorIs(err, errors.HTTPNotFound, "Error should have been a HTTPNotFound but was %T", err)
		_, err = request.Send(&request.Options{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Host: "localhost:9898", Path: "/nowhere"},
		}, nil)
		suite.Require().Error(err, "Should have failed sending a /nowhere request")
		suite.Assert().ErrorIs(err, errors.HTTPNotFound, "Error should have been a HTTPNotFound but was %T", err)
		time.Sleep(400 * time.Millisecond)
		stop <- os.Interrupt
	}()

	err = <-shutdown
	suite.Require().NoError(err, "Failed shutting down the server")
	suite.Assert().False(server.IsReady(), "Server should not be ready anymore")
}

func (suite *ServerSuite) TestCanStartAndShutdownWithProbes() {
	traceenv := os.Getenv("TRACE_PROBE")
	defer os.Setenv("TRACE_PROBE", traceenv)
	os.Setenv("TRACE_PROBE", "true")
	server := NewServer(ServerOptions{
		Port:      9898,
		ProbePort: 9899,
		Logger:    suite.Logger,
	})
	suite.Require().NotNil(server, "Server should not be nil")
	shutdown, stop, err := server.Start(context.Background())
	suite.Require().NoError(err, "Failed starting the server")

	go func() {
		time.Sleep(100 * time.Millisecond)
		suite.Assert().True(server.IsReady(), "Server should be ready")
		_, err = request.Send(&request.Options{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Host: "localhost:9899", Path: "/healthz/readiness"},
		}, nil)
		suite.Require().NoError(err, "Failed sending a health request")
		// Make server not ready
		atomic.StoreInt32(&server.healthStatus, 0)
		_, err = request.Send(&request.Options{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Host: "localhost:9899", Path: "/healthz/readiness"},
		}, nil)
		suite.Require().Error(err, "Should have failed sending a health request")
		suite.Assert().ErrorIs(err, errors.HTTPServiceUnavailable, "Error should have been a HTTPServiceUnavailable but was %T", err)
		time.Sleep(400 * time.Millisecond)
		stop <- os.Interrupt
	}()

	err = <-shutdown
	suite.Require().NoError(err, "Failed shutting down the server")
	suite.Assert().False(server.IsReady(), "Server should not be ready anymore")
}

func (suite *ServerSuite) TestShouldFailStartingWithInvalidPort() {
	server := NewServer(ServerOptions{
		Port: 0,
	})
	suite.Require().NotNil(server, "Server should not be nil")
	shutdown, _, err := server.Start(context.Background())
	suite.Require().Error(err, "Failed starting the server")
	suite.Assert().Nil(shutdown, "Shutdown channel should be nil")
}

func (suite *ServerSuite) TestShouldFailStartingWithInvalidProbePort() {
	server := NewServer(ServerOptions{
		ProbePort: 12,
	})
	suite.Require().NotNil(server, "Server should not be nil")
	shutdown, _, err := server.Start(context.Background())
	suite.Require().Error(err, "Failed starting the server")
	suite.Assert().Nil(shutdown, "Shutdown channel should be nil")
}
