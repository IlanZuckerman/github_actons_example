package diagnostics

import (
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"sync"

	gops "github.com/google/gops/agent"
	opentracing "github.com/opentracing/opentracing-go"
	jaeger "github.com/uber/jaeger-client-go"
	jaegerConfig "github.com/uber/jaeger-client-go/config"

	"github.com/rapid7/csp-cwp-common/pkg/agent_libraries/logger"
)

//TODO: deprecated, remove with config-loader
const (
	globalEnableEnvVarName = "ALCIDE_RUNTIME_DIAGNOSTICS_ENABLED"
	enableEnvVarPrefix     = "ALCIDE_RUNTIME_ENABLE_"
)

type Diagnostics interface {
	//UpdateConfig used for notificationEngine config updates
	UpdateConfig(RuntimeDiagnosticsConfig)
	//Close is a closer function that should be deferred in the main function of the agent for grace shutdown
	Close()
}

type RuntimeDiagnosticsConfig struct {
	//the name to distinguish between the different agents
	ComponentName string
	EnablePprof   bool
	EnableGops    bool
	EnableJaeger  bool
	//string port, usually 6060
	PprofPort string
}

type diagnostics struct {
	configLock sync.Mutex
	log        logger.Logger

	jaegerCloser  io.Closer
	pprofServer   *http.Server
	isGopsEnabled bool
}

func (d *diagnostics) Close() {
	d.safeCloseJaeger() //nolint - fix it
	d.safeCloseGops()
	d.safeClosePprof() //nolint - fix it
}

func (d *diagnostics) UpdateConfig(conf RuntimeDiagnosticsConfig) {
	d.configLock.Lock()
	defer d.configLock.Unlock()

	//Update PProf
	switch isChangeInConfig(d.pprofServer, conf.EnablePprof) {
	case enable:
		d.safeClosePprof() //nolint - fix it
		d.initCPUProfiler(conf)
	case disable:
		d.safeClosePprof() //nolint - fix it
	}

	//Update GOPS
	switch isChangeInConfig(d.isGopsEnabled, conf.EnableGops) {
	case enable:
		d.safeCloseGops()
		d.initRuntimeGopsDiagnostics() //nolint - fix it
	case disable:
		d.safeCloseGops()
	}

	//Update Jaeger
	switch isChangeInConfig(d.jaegerCloser, conf.EnableJaeger) {
	case enable:
		d.safeCloseJaeger() //nolint - fix it
		d.initJaegerCollector(conf)
	case disable:
		d.safeCloseJaeger() //nolint - fix it
	}
}

// initJaeger returns an instance of Jaeger Tracer that samples 100% of traces and logs all spans to stdout.
func (d *diagnostics) initJaegerCollector(conf RuntimeDiagnosticsConfig) {
	//TODO: deprecated, remove with config-loader
	if strings.ToLower(os.Getenv(globalEnableEnvVarName)) != "true" &&
		strings.ToLower(os.Getenv(enableEnvVarPrefix+"JAEGER")) != "true" &&
		!conf.EnableJaeger {
		return
	}

	cfg := &jaegerConfig.Configuration{
		ServiceName: conf.ComponentName,
		Sampler: &jaegerConfig.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &jaegerConfig.ReporterConfig{
			LogSpans: true,
		},
	}

	tracer, closer, err := cfg.NewTracer(jaegerConfig.Logger(jaeger.StdLogger))
	if err != nil {
		d.log.Errorf("ERROR: cannot init Jaeger: %v\n", err)
	}
	d.jaegerCloser = closer
	d.log.Info("successfully initialized Jaeger")
	opentracing.SetGlobalTracer(tracer)

}

func (d *diagnostics) safeCloseJaeger() error {
	if d.jaegerCloser == nil {
		return nil
	}
	opentracing.SetGlobalTracer(nil)
	err := d.jaegerCloser.Close()
	d.jaegerCloser = nil
	return err
}

func (d *diagnostics) initRuntimeGopsDiagnosticsFromEnv(conf RuntimeDiagnosticsConfig) {
	//TODO: deprecated, remove with config-loader
	if strings.ToLower(os.Getenv(globalEnableEnvVarName)) != "true" &&
		strings.ToLower(os.Getenv(enableEnvVarPrefix+"GOPS")) != "true" &&
		!conf.EnableGops {
		return
	}
	d.log.Infof("running GOPS diagnostics")
	err := d.initRuntimeGopsDiagnostics()
	if err != nil {
		fmt.Println("*** Could not enable gops runtime diagnostics ***")
	}
}

func (d *diagnostics) initRuntimeGopsDiagnostics(configDir ...string) error {
	configs := gops.Options{
		ShutdownCleanup: true, // automatically closes on os.Interrupt
	}
	if len(configDir) > 0 {
		configs.ConfigDir = configDir[0]
	}
	err := gops.Listen(configs)
	if err != nil {
		d.isGopsEnabled = true
	}
	fmt.Println("*** Running with runtime diagnostics endpoints ***")

	return err
}

func (d *diagnostics) safeCloseGops() {
	//GOPS close is safe no need to check
	gops.Close()
	d.isGopsEnabled = false
}

func (d *diagnostics) initCPUProfiler(conf RuntimeDiagnosticsConfig) {
	//TODO: deprecated, remove with config-loader
	if strings.ToLower(os.Getenv(globalEnableEnvVarName)) != "true" &&
		strings.ToLower(os.Getenv(enableEnvVarPrefix+"PPROF")) != "true" &&
		!conf.EnablePprof {
		return
	}

	profsrv := http.NewServeMux()

	profsrv.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	profsrv.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	profsrv.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	profsrv.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	profsrv.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	server := &http.Server{Addr: "localhost:" + conf.PprofPort, Handler: profsrv}
	go func() {
		d.log.Infof("%v", server.ListenAndServe())
	}()
	d.pprofServer = server
	d.log.Infof("Running with pprof enabled http://localhost:%s/debug/pprof/", conf.PprofPort)
}

func (d *diagnostics) safeClosePprof() error {
	if d.pprofServer == nil {
		return nil
	}
	err := d.pprofServer.Close()
	return err
}

const (
	enable     = "ENABLE"
	disable    = "DISABLE"
	unchanging = "UNCHANGING"
)

func isChangeInConfig(existing interface{}, toEnable bool) string {
	switch {
	case existing != nil && toEnable:
		return unchanging
	case existing == nil && !toEnable:
		return unchanging
	case existing == nil && toEnable:
		return enable
	case existing != nil && !toEnable:
		return disable
	}
	return unchanging
}

//NewRuntimeDiagnostics is the common diagnostics for all Alcide components
//the global enable is using the env variable ALCIDE_RUNTIME_DIAGNOSTICS_ENABLED if set to "True"
//you can manually activate each tracing using specifying env vars.
func NewRuntimeDiagnostics(params RuntimeDiagnosticsConfig, l logger.Logger) Diagnostics {
	d := &diagnostics{
		log: l,
	}
	d.initRuntimeGopsDiagnosticsFromEnv(params)
	d.initCPUProfiler(params)
	d.initJaegerCollector(params)

	return d
}
