package logger

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/rapid7/csp-cwp-common/pkg/proto/logging"
)

type LoggerTestSuite struct {
	suite.Suite
}

// Make sure that VariableThatShouldStartAtFive is set to five
// before each test
func (suite *LoggerTestSuite) SetupTest() {
}

func (suite *LoggerTestSuite) TearDownTest() {
}

func (suite *LoggerTestSuite) TestLogger__Basic() {
	m, err := NewManager(ManagerConfig{})
	suite.Error(err, "NewManager should not receive nil")
	suite.Nil(m, "manager should not be created with nil config")

	m, err = NewManager(NewDefaultConfig())
	require.NoError(suite.T(), err, "Failed to create logger")

	//TODO discuss with amit vs processorInterface.Shutdown()
	defer m.Close()

	l := m.Default()
	suite.NotNil(l, "Failed to obtain default logger")

	l.SetTraceLevel(DebugLevel)

	l.Debugf("test %s", "with arg")
	l.Debug("Hello 1")
	l.Info("Hello 1")
	l.Warn("Hello 1")
	l.Error("Hello 1")

	sub := m.GetOrCreateLogger("foo").GetOrCreateSubLogger("bar")
	suite.NotNil(sub, "logger is already existing")
	sub.Debug("Hello 1")
	sub.Info("Hello 1")
	sub.Warn("Hello 1")
	sub.Error("Hello 1")
	fooSub := m.GetOrCreateLogger("foo")
	suite.NotNil(fooSub, "error reaching existing logger")
	barSub := fooSub.GetOrCreateSubLogger("bar")
	suite.NotNil(barSub, "could not find existing logger")

	l.SetTraceLevel(InfoLevel)
	youShouldntSeeThis := "Hello - You shouldn't see this"
	output, err := suite.GetStdoutStringFromFunc(sub.Debugf, youShouldntSeeThis)
	suite.NoError(err)
	suite.Equal("", output)

	youShouldSeeThis := "Hello - You should see this"
	output2, err := suite.GetStdoutStringFromFunc(sub.Infof, youShouldSeeThis)
	suite.NoError(err)
	suite.Contains(output2, youShouldSeeThis)

	sub2 := m.GetOrCreateLogger("another").GetOrCreateSubLogger("foofoo").GetOrCreateSubLogger("barbar")
	sub2.SetTraceLevel(DebugLevel)
	sub2.Debug("Hello - You should see this")
	sub2.Info("Hello -  You should see this")

	fmt.Print(m.GetFormattedLoggerNames())
	m.ResetTraceLevel(DebugLevel)
	fmt.Print(m.GetFormattedLoggerNames())

	m.Close()
}

func (suite *LoggerTestSuite) TestLogger__Reset() {
	m, err := NewManager(NewDefaultConfig())
	require.NoError(suite.T(), err, "Failed to create logger")

	l := m.Default()
	suite.NotNil(l, "Failed to obtain default logger")

	l.SetTraceLevel(DebugLevel)
	sub := l.GetOrCreateSubLogger("foo").GetOrCreateSubLogger("bar")
	suite.Equal(sub.GetTraceLevel(), DebugLevel)

	m.ResetTraceLevel(InfoLevel)
	suite.Equal(sub.GetTraceLevel(), InfoLevel)

	l.Debug("Hello 1 - should not see this")
	l.Info("Hello 1")
	l.Warn("Hello 1")
	l.Error("Hello 1")
	sub.Debug("Hello 1 - should not see this")
	sub.Info("Hello 1")
	sub.Warn("Hello 1")
	sub.Error("Hello 1")

	fmt.Print(m.GetFormattedLoggerNames())
}

func (suite *LoggerTestSuite) TestLogger__WithFileAndPanic() {
	testConf := NewDefaultConfig()
	testConf.FileLogging = logging.FileLogging{}
	m, err := NewManager(testConf)
	require.NoError(suite.T(), err, "Failed to create logger")

	tmpdir, err := ioutil.TempDir(os.TempDir(), "unit")
	suite.NoError(err, "Failed to create temp dir")
	fmt.Println("Using ", tmpdir)

	testFileLogging := logging.FileLogging{
		MaxAge:     1,
		MaxSizeMB:  1,
		MaxBackups: 2,
		Dir:        tmpdir,
	}
	confUpdate := newConfigUpdate(map[string]zapcore.Level{"": zapcore.DebugLevel}, testConf.EnableStdoutLogger, testFileLogging, testConf.SentryConfig)
	m.UpdateConfig(confUpdate)
	err2 := wait.PollImmediate(10*time.Millisecond, 1*time.Second, func() (bool, error) {
		lvl, err := m.GetTraceLevel("")
		if lvl == DebugLevel {
			return true, err
		}
		return false, err
	})
	require.NoError(suite.T(), err2, "failed updating config")

	l := m.Default()
	suite.NotNil(l, "Failed to obtain default logger")

	l.SetTraceLevel(DebugLevel)
	l.Debug("Debug")
	l.Info("Info")
	l.Warn("Warn")
	l.Error("Error")

	fmt.Println("Stdout")
	fmt.Fprintf(os.Stderr, "Stderr")

	l.Debug("Last")

	//Allow console output to sync to file
	time.Sleep(time.Millisecond * 300)

	data, err := ioutil.ReadFile(fmt.Sprintf("%v/%v.log", tmpdir, filepath.Base(os.Args[0])))
	suite.NoError(err, "Failed to open log file (%v)", err)

	if !strings.Contains(string(data), "Stdout") {
		suite.Fail("Failed to see Stdout redirected to file", string(data))
	}

	if !strings.Contains(string(data), "Stderr") {
		suite.Fail("Failed to see Stderr redirected to file", string(data))
	}

	defer func() {
		if _ = recover(); false {
			suite.Fail("Expected a panic to recover from")
		}

		err := exec.Command("rm", "-rf", tmpdir).Run()
		require.NoError(suite.T(), err, "could not delete tmp dir")

	}()

	panic("Test Panic")
}

func (suite *LoggerTestSuite) TestLogger__ConfigUpdate() {
	m, err := NewManager(NewDefaultConfig())
	require.NoError(suite.T(), err, "Failed to create logger")

	sub := m.GetOrCreateLogger("foo").GetOrCreateSubLogger("bar")
	initLevel := sub.GetTraceLevel()
	m.UpdateConfig(newConfigUpdate(map[string]zapcore.Level{"foo": zapcore.DebugLevel}, true, logging.FileLogging{}, SentryConfig{}))
	//for concurrency sync
	err2 := wait.PollImmediate(10*time.Millisecond, 1*time.Second, func() (bool, error) {
		if initLevel == sub.GetTraceLevel() {
			return true, nil
		}
		return false, nil
	})
	suite.NoError(err2, "config update did not change level")

}

func (suite *LoggerTestSuite) TestLogger__StdoutConfigUpdate() {
	testConf := NewDefaultConfig()
	testConf.FileLogging = logging.FileLogging{}
	m, err := NewManager(testConf)
	require.NoError(suite.T(), err, "Failed to create logger")

	fooLogger := m.GetOrCreateLogger("foo")

	testString := "capturing output to stdout"
	fooLogger.Info(testString)
	s, err2 := suite.GetStdoutStringFromFunc(fooLogger.Infof, testString)
	suite.NoError(err2)
	suite.Contains(s, testString, "could not find required string in stdout")

	//Test disabled stdout logger
	confUpdate := newConfigUpdate(map[string]zapcore.Level{"": zapcore.DebugLevel}, false, testConf.FileLogging, testConf.SentryConfig)
	m.UpdateConfig(confUpdate)
	err3 := wait.PollImmediate(10*time.Millisecond, 1*time.Second, func() (bool, error) {
		lvl, err := m.GetTraceLevel("")
		if lvl == DebugLevel {
			return true, err
		}
		return false, err
	})
	require.NoError(suite.T(), err3, "failed updating config")

	testToNilString := "not capturing output to disabled stdout"
	nilString, err4 := suite.GetStdoutStringFromFunc(fooLogger.Infof, testToNilString)
	suite.NoError(err4)
	suite.Equal(nilString, "", "stdout should be nil and not %s", nilString)
}

// func (suite *LoggerTestSuite) TestLogger__SubLoggerDepth() {
// 	suite.Fail("add test")
// }

func (suite *LoggerTestSuite) GetStdoutStringFromFunc(writeFunc func(string, ...interface{}), s string, args ...interface{}) (string, error) {
	//Test enabled stdout logger
	stdoutBackup := os.Stdout // keep backup of the real stdout
	r, w, err := os.Pipe()
	require.NoError(suite.T(), err, "Could not capture stdout")
	os.Stdout = w
	defer func() { os.Stdout = stdoutBackup }()

	writeFunc(s, args...)

	ch := make(chan string)
	//defer close(ch)
	go getStdoutToChannel(suite.T(), r, ch)

	closeErr := w.Close()
	suite.NoError(closeErr)

	after := time.After(5 * time.Second)
	select {
	case outString := <-ch:
		return outString, nil
	case <-after:
		return "", fmt.Errorf("could not capture stdout- reached timeout")
	}
}

func getStdoutToChannel(t *testing.T, r *os.File, ch chan string) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	require.NoError(t, err)
	ch <- buf.String()
}

func newConfigUpdate(logTraceLevel map[string]zapcore.Level, stdOutLogger bool, fileConfig logging.FileLogging, sentryConfig SentryConfig) ConfigUpdate {
	c := ConfigUpdate{
		LogTraceLevel: logTraceLevel,
	}

	c.EnableStdoutLogger = stdOutLogger

	c.FileConfig = fileConfig

	c.SentryConfig = sentryConfig

	return c
}

func TestLogger__RUN(t *testing.T) {
	crt := new(LoggerTestSuite)
	suite.Run(t, crt)
}
