//go:build !darwin

package wess

import (
	"context"
	"os"

	"github.com/gildas/go-errors"
)

func (suite *ServerSuite) TestShouldFailStartingWithInvalidPort() {
	server := NewServer(ServerOptions{
		Port:   12,
		Logger: suite.Logger,
	})
	suite.Require().NotNil(server, "Server should not be nil")
	shutdown, stop, err := server.Start(context.Background())
	if err == nil {
		stop <- os.Interrupt
	}
	suite.Require().Error(err, "Should have failed starting the server")
	suite.Logger.Errorf("Expected Error:", err)
	suite.Assert().ErrorIs(err, errors.RuntimeError, "Error should have been a RuntimeError but was %T", err)
	suite.Assert().Nil(shutdown, "Shutdown channel should be nil")
}

func (suite *ServerSuite) TestShouldFailStartingWithInvalidProbePort() {
	server := NewServer(ServerOptions{
		Port:      9898,
		ProbePort: 15,
		Logger:    suite.Logger,
	})
	suite.Require().NotNil(server, "Server should not be nil")
	shutdown, stop, err := server.Start(context.Background())
	if err == nil {
		stop <- os.Interrupt
	}
	suite.Require().Error(err, "Should have failed starting the server")
	suite.Logger.Errorf("Expected Error:", err)
	suite.Assert().ErrorIs(err, errors.RuntimeError, "Error should have been a RuntimeError but was %T", err)
	suite.Assert().Nil(shutdown, "Shutdown channel should be nil")
}
