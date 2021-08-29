//TODO: remove with processor integration
package logger

import (
	"go.uber.org/zap/zapcore"

	"github.com/rapid7/csp-cwp-common/pkg/proto/logging"
)

type ConfigUpdate struct {
	LogTraceLevel      map[string]zapcore.Level
	EnableStdoutLogger bool
	FileConfig         logging.FileLogging
	SentryConfig       SentryConfig
}
