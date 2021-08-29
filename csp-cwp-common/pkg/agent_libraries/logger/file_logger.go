package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	"go.uber.org/atomic"
	"go.uber.org/zap/zapcore"

	"github.com/rapid7/csp-cwp-common/pkg/proto/logging"
	"github.com/rapid7/csp-cwp-common/pkg/version"
)

type rollingFileWriter struct {
	online           atomic.Bool
	logger           *lumberjack.Logger
	stopStdoutToFile chan struct{}
	stopStderrToFile chan struct{}
}

var _ zapcore.WriteSyncer = &rollingFileWriter{}

func (l *rollingFileWriter) Sync() error {
	if !l.online.Load() {
		return nil
	}

	return nil
}

func (l *rollingFileWriter) Write(p []byte) (n int, err error) {
	if !l.online.Load() {
		return len(p), nil
	}

	return l.logger.Write(p)
}

func (l *rollingFileWriter) Close() error {
	if !l.online.Load() {
		return nil
	}
	l.online.Store(false)
	time.Sleep(2 * time.Second) //Yield

	close(l.stopStdoutToFile)
	close(l.stopStderrToFile)
	err := l.logger.Close()
	return err
}

func (l *rollingFileWriter) enableFileLogger(config logging.FileLogging) {

	if l.online.Load() {
		fmt.Println("Rolling File Logger Already Online")
		return
	}

	var dir string

	if config.Dir == "" {
		dir = os.TempDir()
	}
	if !strings.HasSuffix(config.Dir, string(os.PathSeparator)) {
		dir = fmt.Sprintf("%v%c", config.Dir, os.PathSeparator)
	}

	logger := &lumberjack.Logger{
		Filename:   fmt.Sprintf("%v%v.log", dir, filepath.Base(os.Args[0])),
		MaxSize:    int(config.MaxSizeMB),  //megabytes
		MaxAge:     int(config.MaxAge),     //days
		MaxBackups: int(config.MaxBackups), //files
		LocalTime:  config.LocalTimeFileTimestamp,
		Compress:   config.Compress,
	}

	fmt.Printf("==> Logs captured under %v\n", logger.Filename)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func(lj *lumberjack.Logger) {
		stdOut := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		defer r.Close()
		defer w.Close()

		tee := io.TeeReader(r, lj)

		wg.Done()
	stdoutLoop:
		for {
			select {
			case <-l.stopStdoutToFile:
				break stdoutLoop
			default:
				_, err := io.Copy(stdOut, tee)
				if err != nil {
					fmt.Printf("Error copying stdout: %v\n", err)
				}
			}
		}
		os.Stdout = stdOut
	}(logger)

	wg.Add(1)
	go func(lj *lumberjack.Logger) {
		stdErr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w
		defer r.Close()
		defer w.Close()

		tee := io.TeeReader(r, lj)
		wg.Done()
	stderrLoop:
		for {
			select {
			case <-l.stopStderrToFile:
				break stderrLoop
			default:
				_, err := io.Copy(stdErr, tee)
				if err != nil {
					fmt.Println("Error copying stderr:", err)
				}
			}
		}
		os.Stderr = stdErr
	}(logger)

	wg.Wait()
	l.logger = logger

	_, err := l.logger.Write([]byte(fmt.Sprintf("\n\n=== [%v][%v][%v][%v] ===\n\n", os.Getpid(), strings.Join(os.Args, " "), version.GetVersion(), version.GetRepoVersion())))
	if err != nil {
		fmt.Println("Error log writing to file:", err)
	}
	l.online.Store(true)

}

// new Rolling File log
func newRollingFile() *rollingFileWriter {
	return &rollingFileWriter{
		//capacity 1 so channels will not block
		stopStdoutToFile: make(chan struct{}),
		stopStderrToFile: make(chan struct{}),
	}
}
