package mix

import (
	"os"
	"runtime"
	"strings"

	"github.com/grafana/pyroscope-go"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

func (c PyroscopeConfig) resolve(appName string) PyroscopeConfig {
	out := c
	if out.ServerAddress == "" {
		out.ServerAddress = strings.TrimSpace(os.Getenv("PYROSCOPE_SERVER_ADDRESS"))
	}
	if out.ApplicationName == "" {
		out.ApplicationName = strings.TrimSpace(os.Getenv("PYROSCOPE_APPLICATION_NAME"))
	}
	if out.ApplicationName == "" {
		out.ApplicationName = appName
	}
	if out.BasicAuthUser == "" {
		out.BasicAuthUser = os.Getenv("PYROSCOPE_BASIC_AUTH_USER")
	}
	if out.BasicAuthPassword == "" {
		out.BasicAuthPassword = os.Getenv("PYROSCOPE_BASIC_AUTH_PASSWORD")
	}
	if out.AuthToken == "" {
		out.AuthToken = os.Getenv("PYROSCOPE_AUTH_TOKEN")
	}
	if !out.Log && os.Getenv("PYROSCOPE_LOG") == "1" {
		out.Log = true
	}
	// Enabled 是总开关：false 即使配了 ServerAddress / PYROSCOPE_SERVER_ADDRESS 也不启动。
	return out
}

func serviceNameFromResource(res *resource.Resource) string {
	if res == nil {
		return ""
	}
	for _, a := range res.Attributes() {
		if a.Key == semconv.ServiceNameKey {
			return a.Value.AsString()
		}
	}
	return ""
}

func startPyroscope(cfg PyroscopeConfig) (*pyroscope.Profiler, error) {
	if !cfg.DisableMutexProfile {
		runtime.SetMutexProfileFraction(5)
	}
	if !cfg.DisableBlockProfile {
		runtime.SetBlockProfileRate(5)
	}
	pc := pyroscope.Config{
		ApplicationName: cfg.ApplicationName,
		ServerAddress:   cfg.ServerAddress,
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	}
	if cfg.AuthToken != "" {
		pc.AuthToken = cfg.AuthToken
	} else if cfg.BasicAuthUser != "" {
		pc.BasicAuthUser = cfg.BasicAuthUser
		pc.BasicAuthPassword = cfg.BasicAuthPassword
	}
	if cfg.Log {
		pc.Logger = pyroscope.StandardLogger
	}
	return pyroscope.Start(pc)
}
