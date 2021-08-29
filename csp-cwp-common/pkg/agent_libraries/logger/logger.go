package logger

import (
	"fmt"
	"sync"
	"time"

	metrics "github.com/rcrowley/go-metrics"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const maxDepth = 3 //nolint - fix it

type Logger interface {
	//GetOrCreateSubLogger creates and returns a new child logger object. returns nil if exists
	GetOrCreateSubLogger(string) Logger
	//GetSubLoggersNames returns the names of the children only (not recursive)
	GetSubLoggersNames() []string
	//SetTraceLevel recursively sets the level for the logger and its children.
	SetTraceLevel(level zapcore.Level)
	//GetTraceLevel returns the current configured level of the logger
	GetTraceLevel() zapcore.Level

	//Debug writes a log with DebugLevel
	Debug(string, ...zapcore.Field)
	//Debugf formats a format string with the supplied args and write Debug log
	Debugf(format string, args ...interface{})
	//Info writes a log with InfoLevel
	Info(string, ...zapcore.Field)
	//Infof formats a format string with the supplied args and write Info log
	Infof(string, ...interface{})
	//Warn writes a log with WarnLevel
	Warn(string, ...zapcore.Field)
	//Warnf formats a format string with the supplied args and write Warn log
	Warnf(string, ...interface{})
	//Error writes a log with ErrorLevel
	Error(string, ...zapcore.Field)
	//Errorf formats a format string with the supplied args and write Error log
	Errorf(string, ...interface{})
	//Panic writes a log with panicLevel
	Panic(string, ...zapcore.Field)
	//Panicf formats a format string with the supplied args and write Panic log
	Panicf(string, ...interface{})
	//Fatal writes a log with FatalLevel
	Fatal(string, ...zapcore.Field)
	//Fatalf formats a format string with the supplied args and write Fatal log
	Fatalf(string, ...interface{})
}

//logger is a wrapper for zap.SugaredLogger.
type logger struct {
	confLock sync.RWMutex

	name string
	//TODO restrict depth
	subLoggers   map[string]*logger // logger name -> logger
	zapLogger    *zap.Logger
	sentryLogger *sentryWriter
	level        zap.AtomicLevel

	errors    metrics.Counter
	warnnings metrics.Counter
}

func (l *logger) GetOrCreateSubLogger(name string) Logger {
	sub := l.getOrCreateInternalSubLogger(name)
	return sub
}

func (l *logger) GetSubLoggersNames() []string {
	l.confLock.RLock()
	defer l.confLock.RUnlock()
	var loggerNames []string
	for name := range l.subLoggers {
		loggerNames = append(loggerNames, name)
	}
	return loggerNames
}

func (l *logger) GetTraceLevel() zapcore.Level {
	return l.level.Level()
}

func (l *logger) SetTraceLevel(level zapcore.Level) {
	l.confLock.Lock()
	defer l.confLock.Unlock()
	l.level.SetLevel(level)
	for _, sub := range l.subLoggers {
		//TODO dont use recursion
		sub.SetTraceLevel(level)
	}
}

func (l *logger) GetSubLogger(name string) Logger {
	l.confLock.RLock()
	defer l.confLock.RUnlock()
	sub, ok := l.subLoggers[name]
	if !ok {
		return nil
	}
	return sub
}

func (l *logger) isDebugLevel() bool {
	return l.level.Enabled(zapcore.DebugLevel)
}

func (l *logger) Debug(msg string, fields ...zapcore.Field) {
	l.zapLogger.Debug(msg, fields...)
	l.sentryLogger.sendSentryMessage(l.name, zapcore.DebugLevel, msg)
}

func (l *logger) Debugf(format string, args ...interface{}) {
	//optimization to save the overhead of formatting strings with fmt.Sprintf
	if !l.isDebugLevel() {
		return
	}
	msg := fmt.Sprintf(format, args...)
	l.zapLogger.Debug(msg)
	l.sentryLogger.sendSentryMessage(l.name, zapcore.DebugLevel, msg)
}

func (l *logger) Info(msg string, fields ...zapcore.Field) {
	l.zapLogger.Info(msg, fields...)
	l.sentryLogger.sendSentryMessage(l.name, zapcore.InfoLevel, msg)
}

func (l *logger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.zapLogger.Info(msg)
	l.sentryLogger.sendSentryMessage(l.name, zapcore.InfoLevel, msg)
}

func (l *logger) Warn(msg string, fields ...zapcore.Field) {
	l.warnnings.Inc(1)
	l.zapLogger.Warn(msg, fields...)
	l.sentryLogger.sendSentryMessage(l.name, zapcore.WarnLevel, msg)
}

func (l *logger) Warnf(format string, args ...interface{}) {
	l.warnnings.Inc(1)
	msg := fmt.Sprintf(format, args...)
	l.zapLogger.Warn(msg)
	l.sentryLogger.sendSentryMessage(l.name, zapcore.WarnLevel, msg)
}

func (l *logger) Error(msg string, fields ...zapcore.Field) {
	l.errors.Inc(1)
	l.zapLogger.Error(msg, fields...)
	l.sentryLogger.sendSentryMessage(l.name, zapcore.ErrorLevel, msg)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	l.errors.Inc(1)
	msg := fmt.Sprintf(format, args...)
	l.zapLogger.Error(msg)
	l.sentryLogger.sendSentryMessage(l.name, zapcore.ErrorLevel, msg)
}
func (l *logger) Panic(msg string, fields ...zapcore.Field) {
	l.errors.Inc(1)
	l.zapLogger.Panic(msg, fields...)
	l.sentryLogger.sendSentryMessage(l.name, zapcore.PanicLevel, msg)
}

func (l *logger) Panicf(format string, args ...interface{}) {
	l.errors.Inc(1)
	str := fmt.Sprintf(format, args...)
	l.zapLogger.Panic(str)
	l.sentryLogger.sendSentryMessage(l.name, zapcore.PanicLevel, str)
}

func (l *logger) Fatal(msg string, fields ...zapcore.Field) {
	l.errors.Inc(1)
	l.zapLogger.Fatal(msg, fields...)
	l.sentryLogger.sendSentryMessage(l.name, zapcore.FatalLevel, msg)
}

func (l *logger) Fatalf(format string, args ...interface{}) {
	l.errors.Inc(1)
	str := fmt.Sprintf(format, args...)
	l.zapLogger.Fatal(str)
	l.sentryLogger.sendSentryMessage(l.name, zapcore.FatalLevel, str)
}

//TODO: add with telemetry
func (l *logger) ErrorCounter() metrics.Counter {
	return l.errors
}

//TODO: add with telemetry
func (l *logger) WarnCounter() metrics.Counter {
	return l.warnnings
}

func timestampFormat(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("[Jan-02 15:04:05.000000000]"))
}

func newZapLoggerTargets(writers ...zapcore.WriteSyncer) zapcore.WriteSyncer {
	return zapcore.NewMultiWriteSyncer(writers...)
}

//getOrCreateInternalSubLogger returns pointer to logger struct instead of Logger interface for internal purposes.
func (l *logger) getOrCreateInternalSubLogger(name string) *logger {
	//if no name specified return the self logger
	if name == DefaultLoggerName {
		return l
	}

	l.confLock.Lock()
	defer l.confLock.Unlock()

	if subLogger, ok := l.subLoggers[name]; ok {
		return subLogger
	}

	loggerName := fmt.Sprintf("%v.%v", l.name, name)

	sub := &logger{
		name:         loggerName,
		subLoggers:   make(map[string]*logger),
		zapLogger:    l.zapLogger.Named(name),
		level:        l.level,
		errors:       l.errors,    //Point to the parent
		warnnings:    l.warnnings, //Point to the parent
		sentryLogger: l.sentryLogger,
	}

	l.subLoggers[name] = sub
	return sub
}

func newZapLogger(encodeAsJSON bool, level zap.AtomicLevel, output zapcore.WriteSyncer) *zap.Logger {
	encCfg := zapcore.EncoderConfig{
		TimeKey:       "timestamp",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "msg",
		StacktraceKey: "stacktrace",
		//EncodeLevel:    zapcore.LowercaseColorLevelEncoder,
		EncodeTime:     timestampFormat,
		EncodeDuration: zapcore.NanosDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder

	if encodeAsJSON {
		encCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
		encoder = zapcore.NewJSONEncoder(encCfg)
	} else {
		encCfg.EncodeLevel = zapcore.LowercaseColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encCfg)
	}

	l := zap.New(zapcore.NewCore(encoder, output, level), zap.AddCaller(), zap.AddCallerSkip(1))
	return l
}

// new logger by specific log name
func newLogger(config ManagerConfig, stdoutLogger, fileLogger zapcore.WriteSyncer, sentryLogger *sentryWriter) *logger {
	targets := newZapLoggerTargets(stdoutLogger, fileLogger)
	level := zap.NewAtomicLevel()
	level.SetLevel(config.DefaultTraceLevel)
	zapLogger := newZapLogger(
		config.EncodeLogsAsJson,
		level,
		targets,
	)

	zap.RedirectStdLog(zapLogger)

	l := &logger{
		name:         config.Name,
		subLoggers:   make(map[string]*logger),
		zapLogger:    zapLogger.Named(config.Name),
		level:        level,
		errors:       metrics.NewCounter(),
		warnnings:    metrics.NewCounter(),
		sentryLogger: sentryLogger,
	}

	return l
}
