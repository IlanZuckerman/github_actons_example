package logger

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	sentry "github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"

	"github.com/rapid7/csp-cwp-common/pkg/version"
)

const E2ENotificationAgentFilePath = "/opt/alcide/e2e.txt"

//SentryConfig is the initiative configurations for newSentryLogger
type SentryConfig struct {
	//Enable the sentry reporter. If set to false configuration will be saved and will be used when enabled later
	Enable bool
	//minimum logger level to report to sentry
	Level zapcore.Level
	//Data Source Name - the URL for reporting
	Dsn string
	//Start Sentry in Debug Mode
	SetDebug bool
	//build version branch/tag
	Version         string
	OrganizationUid string
	ClusterUid      string
	NodeUid         string
}

type sentryWriter struct {
	sync.RWMutex
	client  *sentry.Client
	options sentry.ClientOptions
	Level   zapcore.Level
	log     *logger
}

//RecoverRepanicSentry makes sure the program will report to sentry in case of panic before exiting the program
//example from https://github.com/getsentry/sentry-go/blob/v0.7.0/example/recover-repanic/main.go
func (s *sentryWriter) RecoverRepanicSentry(repanic bool) {
	defer s.client.Flush(5 * time.Second)
	if err := recover(); err != nil {
		localHub := sentry.NewHub(s.client, sentry.NewScope())
		localHub.Recover(err)
		if repanic {
			panic(err)
		}
	}
}

//updateSentryConfig safely applies new SentryConfig
func (s *sentryWriter) updateSentryConfig(configUpdate SentryConfig) {
	s.Lock()
	defer s.Unlock()
	s.options.Debug = configUpdate.SetDebug
	s.Level = configUpdate.Level
	s.options.Dsn = configUpdate.Dsn
	switch configUpdate.Enable {
	case false:
		s.client = &sentry.Client{}
	case true:
		client, err := sentry.NewClient(s.options)
		if err != nil {
			s.log.Errorf("error creating sentry config from update: %s", err)
			break
		}
		s.client = client
	}
}

//initSentryDiagnostics configures options and start the sentry client accordingly
func (s *sentryWriter) initSentryDiagnostics(conf SentryConfig) {
	sentryOptions := sentry.ClientOptions{
		// Either set environment and release here or set the SENTRY_ENVIRONMENT
		// and SENTRY_RELEASE environment variables.
		Environment: os.Getenv("MANAGEMENT_DOMAIN"),
		ServerName:  "no_node_uid",
		Release:     version.GetVersion(),
		// Enable printing of SDK debug messages.
		// Useful when getting started or trying to figure something out.
		Debug:            conf.SetDebug,
		SampleRate:       1.0,
		AttachStacktrace: true,
	}
	s.options = sentryOptions
	if conf.NodeUid != "" {
		s.options.ServerName = conf.NodeUid
	}

	collector := &sentryCollector{
		sentryFixedTags: make(map[string]string),
	}
	collector.sentryFixedTags["organization"] = conf.OrganizationUid
	collector.sentryFixedTags["cluster"] = conf.ClusterUid
	collector.sentryFixedTags["agent_hostname"] = os.Getenv("HOSTNAME")
	s.options.BeforeSend = collector.diagnosticsBeforeSentrySend
	if !conf.Enable {
		return
	}

	if conf.Dsn == "" {
		s.log.Warnf("No DSN supplied")
		return
	}
	s.options.Dsn = conf.Dsn

	client, err := sentry.NewClient(s.options)
	if err != nil {
		s.log.Errorf("sentry init error: %s", err)
		return
	}
	s.log.Debugf("Sentry client: %v", client)
	s.client = client
	s.log.Infof("Sentry initialized with conf:\n%v", sentryOptions)
	s.sendSentryMessage("init", zapcore.InfoLevel, fmt.Sprintf("Sentry initialized on %s/%s/%s", conf.OrganizationUid, conf.ClusterUid, conf.NodeUid))
}

//sendSentryMessage checks a received level and reports
func (s *sentryWriter) sendSentryMessage(loggerName string, lvl zapcore.Level, msgString string) {
	if s == nil {
		//skip in case sentry wasn't initialized
		return
	}
	s.RLock()
	defer s.RUnlock()
	if s.options.Dsn == "" {
		return
	}
	if s.Level > lvl {
		//Only report Log with level at least high as configured sentry level
		return
	}
	localHub := sentry.NewHub(s.client, sentry.NewScope())
	localHub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetLevel(convertToSentryLevel(lvl))
		scope.SetTag("logger", loggerName)
		loggerShortName := parseLoggerName(loggerName)
		scope.SetFingerprint([]string{loggerShortName, getCallerString(5)})
	})
	s.log.Infof("Sending sentry Message: %s\n%v", msgString, localHub)
	localHub.CaptureMessage(msgString)
}

type sentryCollector struct {
	sentryFixedTags map[string]string
}

//diagnosticsBeforeSentrySend runs as  sentry.ClientOptions.BeforeSend - whenever we decide to report an error just before the actual delivery.
//The purpose of this function is to add the runtime tags which might change according to the error we sent or the time -
//such as the breadcrumbs, the logger name, and test name. In addition, we attach all other tags such as management, org, UIDs, etc...
func (c *sentryCollector) diagnosticsBeforeSentrySend(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
	event.Modules = nil
	//set test tag
	fileBytes, err := ioutil.ReadFile(E2ENotificationAgentFilePath)
	var testTag string
	switch {
	case err != nil:
		testTag = "none"
	case string(fileBytes) == "notest":
		testTag = "notest"
	case string(fileBytes) == "":
		testTag = "notest"
	default:
		testTag = string(fileBytes)
	}
	event.Tags["test_name"] = testTag
	//add fixed tags
	for key, value := range c.sentryFixedTags {
		event.Tags[key] = value
	}
	//set user
	event.User = sentry.User{
		ID: fmt.Sprintf("%v/%v", c.sentryFixedTags["cluster"], c.sentryFixedTags["agent_hostname"]),
	}
	//set logger
	logger, ok := event.Tags["logger"]
	if !ok {
		return event
	}
	event.Logger = logger
	delete(event.Tags, "logger")
	return event
}

func convertToSentryLevel(lvl zapcore.Level) sentry.Level {
	switch lvl {
	case zapcore.DebugLevel:
		return sentry.LevelDebug
	case zapcore.InfoLevel:
		return sentry.LevelInfo
	case zapcore.WarnLevel:
		return sentry.LevelWarning
	case zapcore.ErrorLevel:
		return sentry.LevelError
	case zapcore.PanicLevel:
		return sentry.LevelFatal
	case zapcore.DPanicLevel:
		return sentry.LevelFatal
	case zapcore.FatalLevel:
		return sentry.LevelFatal
	}
	return sentry.LevelError
}

func parseLoggerName(name string) string {
	s := strings.Split(name, ".")
	switch len(s) {
	case 0:
		return ""
	case 1:
		return s[0]
	default:
		return fmt.Sprintf("%v.%v", s[0], s[1])
	}
}

func getCallerString(skip int) string {
	_, filePath, line, ok := runtime.Caller(skip)
	if !ok {
		filePath = "???"
		line = 0
	}
	callerStr := fmt.Sprintf("%s:%d", filePath, line)
	return callerStr
}

func newSentryLogger(config SentryConfig, logger *logger) *sentryWriter {
	if config == (SentryConfig{}) {
		return nil
	}
	s := &sentryWriter{
		log:   logger,
		Level: config.Level,
	}
	s.initSentryDiagnostics(config)
	return s
}
