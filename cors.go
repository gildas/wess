package wess

import "github.com/gildas/go-logger"

// CORS Logger
type CorsLogger struct {
	Logger *logger.Logger
}

func (corsLogger CorsLogger) Printf(format string, args ...interface{}) {
	corsLogger.Logger.Debugf(format, args...)
}
