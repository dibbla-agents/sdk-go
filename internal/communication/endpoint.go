package communication

import (
	"os"
	"strings"
)

// DefaultServerAddress is Dibbla's hosted platform, used when nothing names
// a server.
const DefaultServerAddress = "grpc.dibbla.com:443"

// Environment variables that name the workflow server.
const (
	// ServerAddressEnv and UseTLSEnv are the user's: set them to reach a
	// particular server (local development: localhost:50051).
	ServerAddressEnv = "GRPC_SERVER_ADDRESS"
	UseTLSEnv        = "GRPC_USE_TLS"

	// PlatformServerAddressEnv and PlatformUseTLSEnv are set by the Dibbla
	// platform (deploy-api, DIB-1430) on every tenant container: the gRPC
	// address of the install the worker is deployed on, and whether it
	// speaks TLS. They are why a worker deployed on a self-hosted install
	// reaches that install rather than DefaultServerAddress with no
	// configuration. The platform reserves the DIBBLA_ prefix, so users
	// cannot set these.
	PlatformServerAddressEnv = "DIBBLA_GRPC_SERVER_ADDRESS"
	PlatformUseTLSEnv        = "DIBBLA_GRPC_USE_TLS"
)

// EnvEndpoint is the workflow server as the environment describes it.
type EnvEndpoint struct {
	// Address: GRPC_SERVER_ADDRESS, else DIBBLA_GRPC_SERVER_ADDRESS, else
	// DefaultServerAddress. Blank values count as unset.
	Address string
	// FromPlatform reports that Address came from DIBBLA_GRPC_SERVER_ADDRESS.
	FromPlatform bool
	// UseTLS is GRPC_USE_TLS (nil = unset).
	UseTLS *bool
	// PlatformUseTLS is DIBBLA_GRPC_USE_TLS, and only when Address came from
	// the platform: the hint describes the platform's address, never one the
	// user chose. nil when that is not the case, or the value is not
	// true/1/false/0.
	PlatformUseTLS *bool
}

// EndpointFromEnv reads the workflow server address and TLS settings from
// the environment.
func EndpointFromEnv() EnvEndpoint {
	e := EnvEndpoint{Address: DefaultServerAddress, UseTLS: useTLSFromEnv()}
	if addr := strings.TrimSpace(os.Getenv(ServerAddressEnv)); addr != "" {
		e.Address = addr
	} else if addr := strings.TrimSpace(os.Getenv(PlatformServerAddressEnv)); addr != "" {
		e.Address = addr
		e.FromPlatform = true
		e.PlatformUseTLS = parseTLSHint(os.Getenv(PlatformUseTLSEnv))
	}
	return e
}

// TLS is the TLS choice the environment makes: GRPC_USE_TLS, else the
// platform's hint for its own address. nil = auto-detect (ShouldUseTLS).
func (e EnvEndpoint) TLS() *bool {
	if e.UseTLS != nil {
		return e.UseTLS
	}
	return e.PlatformUseTLS
}

// useTLSFromEnv keeps GRPC_USE_TLS's long-standing reading: any non-empty
// value is a choice, and only "true" or "1" means TLS.
func useTLSFromEnv() *bool {
	v := os.Getenv(UseTLSEnv)
	if v == "" {
		return nil
	}
	useTLS := v == "true" || v == "1"
	return &useTLS
}

// parseTLSHint reads true/1 and false/0; anything else is no hint.
func parseTLSHint(v string) *bool {
	var useTLS bool
	switch strings.TrimSpace(v) {
	case "true", "1":
		useTLS = true
	case "false", "0":
		useTLS = false
	default:
		return nil
	}
	return &useTLS
}
