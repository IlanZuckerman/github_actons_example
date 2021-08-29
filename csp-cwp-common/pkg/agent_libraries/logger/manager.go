package logger

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"

	metrics "github.com/rcrowley/go-metrics"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/rapid7/csp-cwp-common/pkg/proto/logging"
)

const (
	DebugLevel = zapcore.DebugLevel
	InfoLevel  = zapcore.InfoLevel
	WarnLevel  = zapcore.WarnLevel
	ErrorLevel = zapcore.WarnLevel
	PanicLevel = zapcore.PanicLevel
	FatalLevel = zapcore.FatalLevel
)

const DefaultLoggerName = ""

type Manager interface {
	//Default returns the default logger (name="")
	Default() Logger
	//GetOrCreateLogger creates new logger in the root of the manager loggers or returns the existing one
	GetOrCreateLogger(logger string) Logger
	//UpdateConfig used for notificationEngine config updated
	UpdateConfig(update ConfigUpdate)
	//GetTraceLevel returns the trace level of a specific logger.
	//returns error if such logger does not exist
	//logger name must be in period separated string e.g: "foo.bar"
	GetTraceLevel(name string) (zapcore.Level, error)
	//SetTraceLevel recursively sets the level of logger and its sub-loggers to the provided one
	//name must be in period separated string e.g: "foo.bar">
	SetTraceLevel(name string, level zapcore.Level) error
	//ResetTraceLevel sets the trace level to ALL loggers
	ResetTraceLevel(level zapcore.Level)
	//TODO: add with telemetry
	//GetMetrics() metrics.Registry

	//GetFormattedLoggerNames returns formatted version to show the current topics and string
	GetFormattedLoggerNames() string
	//Close closes the file logger safely and checks for panic for reporting to sentry.
	//The method should be deferred in the main function of the program
	Close()
}

//ManagerConfig holds logger manager initiative configurations.
//NewDefaultConfig creates default config for that purpose
type ManagerConfig struct {
	logging.FileLogging
	SentryConfig

	//DefaultTraceLevel is used when creating a new Logger.
	//sub-loggers will be created with their parent Logger trace level
	DefaultTraceLevel zapcore.Level
	// EncodeLogsAsJson makes the log framework log JSON
	EncodeLogsAsJson bool
	//TraceManager Name
	Name               string
	EnableStdoutLogger bool
}

type manager struct {
	confLock sync.RWMutex
	loggers  map[string]*logger // logger name -> logger
	//TODO: redefine with telemetry
	//stats   metrics.Registry

	stdoutWriter *stdoutWriter
	fileWriter   *rollingFileWriter
	sentryWriter *sentryWriter

	configCh chan ConfigUpdate
	conf     ManagerConfig
}

func (m *manager) GetOrCreateLogger(name string) Logger {
	m.confLock.RLock()
	if logger, ok := m.loggers[name]; ok {
		m.confLock.RUnlock()
		return logger
	}
	m.confLock.RUnlock()
	l := m.loggers[DefaultLoggerName].getOrCreateInternalSubLogger(name)
	//TODO: redefine with telemetry
	//m.stats.Register(fmt.Sprintf("%s_error_count", name), l.ErrorCounter())
	//m.stats.Register(fmt.Sprintf("%s_warn_count", name), l.WarnCounter())

	l.SetTraceLevel(m.conf.DefaultTraceLevel)
	m.confLock.Lock()
	defer m.confLock.Unlock()
	m.loggers[name] = l

	return l
}

func (m *manager) UpdateConfig(config ConfigUpdate) {
	m.configCh <- config
}

func (m *manager) waitForConfigUpdate() {
	for conf := range m.configCh {
		m.safeUpdateConfig(conf)
	}
}

func (m *manager) safeUpdateConfig(conf ConfigUpdate) {
	m.confLock.Lock()
	defer m.confLock.Unlock()
	if conf.FileConfig != (logging.FileLogging{}) {
		if m.fileWriter.logger != nil {
			m.fileWriter.Close()
		}
		m.fileWriter.enableFileLogger(conf.FileConfig)
	}
	if conf.SentryConfig != (SentryConfig{}) {
		m.sentryWriter.updateSentryConfig(conf.SentryConfig)
	}

	m.stdoutWriter.isEnabled = conf.EnableStdoutLogger

	if conf.LogTraceLevel != nil {
		for logName, level := range conf.LogTraceLevel {
			l, ok := m.loggers[logName]
			if ok {
				l.SetTraceLevel(level)
			}
		}
	}
}

func (m *manager) SetTraceLevel(name string, level zapcore.Level) error {
	m.confLock.Lock()
	defer m.confLock.Unlock()

	namesHierarchy := strings.Split(name, ".")
	if logger, ok := m.loggers[name]; ok {
		return localSetTraceLevel(logger, level, namesHierarchy)
	}

	return fmt.Errorf("NewLogger %s not found", name)
}

//localSetTraceLevel should be used only when manager is locked for this purpose
func localSetTraceLevel(logger *logger, level zapcore.Level, names []string) error {
	if len(names) > 1 {
		return localSetTraceLevel(logger, level, names[1:])
	}
	if subLogger, ok := logger.subLoggers[names[0]]; ok {
		subLogger.SetTraceLevel(level)
		return nil
	}
	return fmt.Errorf("logger %s not fount as subLogger of %s", names[0], names[1])
}

func (m *manager) GetTraceLevel(name string) (zapcore.Level, error) {
	m.confLock.RLock()
	defer m.confLock.RUnlock()

	if logger, ok := m.loggers[name]; ok {
		return logger.level.Level(), nil
	}

	return 0, fmt.Errorf("logger %s not found", name)
}

func (m *manager) Default() Logger {
	if logger, ok := m.loggers[DefaultLoggerName]; ok {
		return logger
	}

	//Should never happen
	return nil
}

func (m *manager) ResetTraceLevel(level zapcore.Level) {
	m.confLock.Lock()
	defer m.confLock.Unlock()

	m.conf.DefaultTraceLevel = level

	for _, logger := range m.loggers {
		logger.SetTraceLevel(m.conf.DefaultTraceLevel)
	}
}

func (m *manager) GetMetrics() metrics.Registry {
	//TODO: redefine with telemetry
	//return m.stats
	return nil
}

func (m *manager) GetFormattedLoggerNames() string {
	m.confLock.RLock()
	defer m.confLock.RUnlock()

	str := bytes.NewBufferString("")
	out := tabwriter.NewWriter(str, 1, 10, 5, ' ', tabwriter.Debug)

	//Header
	fmt.Fprintln(out, "TOPIC\t", "LEVEL")
	fmt.Fprintln(out, "-----\t", "-----")

	topics := make([]string, len(m.loggers))
	for name := range m.loggers {
		topics = append(topics, name)
	}

	sort.Strings(topics)

	for _, name := range topics {
		if name != "" {
			logger := m.loggers[name]
			fmt.Fprintln(out, name, "\t", logger.level.Level().String())
		}
	}

	out.Flush()

	return str.String()
}

func (m *manager) Close() {
	m.confLock.Lock()
	defer m.confLock.Unlock()

	if m.fileWriter != nil {
		err := m.fileWriter.Close()
		if err != nil {
			m.Default().Errorf("Could not close file writer: %s", err)
		}
		m.fileWriter = nil
	}
	if m.sentryWriter != nil && m.conf.SentryConfig.Enable {
		m.sentryWriter.RecoverRepanicSentry(true)
	}
}

type stdoutWriter struct {
	isEnabled bool
}

func (sl *stdoutWriter) Write(p []byte) (n int, err error) {
	if !sl.isEnabled {
		return 0, nil
	}
	return os.Stdout.Write(p)
}

func (sl *stdoutWriter) Sync() error {
	return os.Stdout.Sync()
}

//NewDefaultConfig returns default suggested config for
//TODO: set as default in protobuf
func NewDefaultConfig() ManagerConfig {
	c := ManagerConfig{
		DefaultTraceLevel:  InfoLevel,
		EncodeLogsAsJson:   false,
		Name:               "app",
		EnableStdoutLogger: true,
		SentryConfig:       SentryConfig{},
		FileLogging: logging.FileLogging{
			Dir:                    "./",
			MaxSizeMB:              10,
			MaxBackups:             0,
			MaxAge:                 7,
			Compress:               false,
			LocalTimeFileTimestamp: false,
		},
	}

	return c
}

func NewManager(conf ManagerConfig) (Manager, error) {
	if conf == (ManagerConfig{}) {
		return nil, fmt.Errorf("no logger config supplied")
	}

	m := &manager{
		loggers: make(map[string]*logger),
		//TODO: redefine with telemetry
		//stats:      metrics.NewPrefixedRegistry(fmt.Sprintf("%s_trace_", strings.ToLower(conf.Name))),
		stdoutWriter: &stdoutWriter{conf.EnableStdoutLogger},
		fileWriter:   newRollingFile(),
		conf:         conf,
		configCh:     make(chan ConfigUpdate, 1),
	}

	if conf.FileLogging != (logging.FileLogging{}) {
		m.fileWriter.enableFileLogger(conf.FileLogging)
	}

	l := newLogger(conf, m.stdoutWriter, m.fileWriter, nil)
	//TODO: redefine with telemetry
	//m.stats.Register("default_error_count", l.ErrorCounter())
	//m.stats.Register("default_warn_count", l.WarnCounter())

	sentryLoggerClient := newSentryLogger(conf.SentryConfig, l.getOrCreateInternalSubLogger("sentry"))
	if sentryLoggerClient == nil {
		m.conf.SentryConfig.Enable = false
	}
	l.sentryLogger = sentryLoggerClient
	m.sentryWriter = sentryLoggerClient

	m.loggers[DefaultLoggerName] = l
	zap.ReplaceGlobals(l.zapLogger)
	go m.waitForConfigUpdate()

	return m, nil
}
