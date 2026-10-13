package sdk

import (
	"os"
	"strconv"

	"github.com/dibbla-agents/sdk-go/internal/communication"
)

// Config holds the configuration for the SDK server
type Config struct {
	ServerName   string
	CodexEnvPath string

	// Communication configuration
	CommunicationMode string // will be forced to "grpc"
	// GrpcServerAddress is the gRPC workflow server. Precedence:
	// WithGrpcServerAddress, then GRPC_SERVER_ADDRESS, then
	// DIBBLA_GRPC_SERVER_ADDRESS (set by the Dibbla platform to the install
	// the worker is deployed on, DIB-1430), then grpc.dibbla.com:443.
	GrpcServerAddress string
	ServerApiToken    string // API token for authentication
	// IdentityTokenFile points at a projected workload-identity token file
	// (DIB-202). Precedence: ServerApiToken (explicit key) wins; otherwise
	// this file is used when present; the Dibbla platform sets
	// DIBBLA_IDENTITY_TOKEN_FILE and mounts the default path, so in-cluster
	// workers need NO configuration at all. The file is re-read at every
	// (re)connect, following kubelet rotation.
	IdentityTokenFile string
	OrgID             string // optional: pin registration to a specific org (multi-org users)
	// GrpcUseTLS: WithGrpcTLS or GRPC_USE_TLS. nil = the platform's
	// DIBBLA_GRPC_USE_TLS when GrpcServerAddress is the platform's address,
	// otherwise auto-detect based on address.
	GrpcUseTLS *bool
	// GrpcTLSInsecureSkipVerify disables TLS certificate verification. Only
	// takes effect when TLS is in use. Insecure: keeps the connection encrypted
	// but does not validate the server certificate. Use only for self-signed or
	// otherwise untrusted certificates in controlled environments.
	GrpcTLSInsecureSkipVerify bool

	// Handler/dispatcher configuration
	HandlersConcurrency  int
	IncomingEventsBuffer int

	// gRPC lifecycle configuration
	GrpcReconnectIntervalSec   int
	GrpcHealthcheckIntervalSec int
	PingIntervalSec            int // Interval for sending ping messages (0 = disabled)

	// HTTP/2 keepalive ping interval and ack timeout (seconds). 0 = SDK
	// defaults (5m / 20s). See WithGrpcKeepalive for constraints.
	GrpcKeepaliveTimeSec    int
	GrpcKeepaliveTimeoutSec int

	// platformGrpcAddress and platformGrpcUseTLS remember the platform's
	// DIBBLA_GRPC_SERVER_ADDRESS / DIBBLA_GRPC_USE_TLS when the address came
	// from there, so the TLS hint applies to that address only — never to one
	// chosen with an option (which clears them) or set on the Config later.
	platformGrpcAddress string
	platformGrpcUseTLS  *bool
}

// Option is a functional option for configuring the SDK
type Option func(*Config)

// defaultConfig returns the default configuration, reading from environment variables
func defaultConfig() *Config {
	serverName := getEnvWithDefault("SERVER_NAME", "codex-go-worker")
	codexEnvPath := getEnvWithDefault("CODEX_ENV_PATH", "")
	communicationMode := getEnvWithDefault("COMMUNICATION_MODE", "grpc")
	// GRPC_SERVER_ADDRESS, else the platform's DIBBLA_GRPC_SERVER_ADDRESS,
	// else grpc.dibbla.com:443; GRPC_USE_TLS, and the platform's TLS hint.
	endpoint := communication.EndpointFromEnv()
	serverApiToken := getEnvWithDefault("SERVER_API_TOKEN", "")
	identityTokenFile := getEnvWithDefault("DIBBLA_IDENTITY_TOKEN_FILE", "")
	orgID := getEnvWithDefault("SERVER_ORG_ID", "")

	var platformGrpcAddress string
	if endpoint.FromPlatform {
		platformGrpcAddress = endpoint.Address
	}

	// TLS certificate verification can be skipped via env (insecure)
	insecureSkipVerify := false
	if v := os.Getenv("GRPC_TLS_INSECURE_SKIP_VERIFY"); v == "true" || v == "1" {
		insecureSkipVerify = true
	}

	// HTTP/2 keepalive tuning via env (0 = SDK defaults). Lets deployed
	// workers opt into faster dead-connection detection without a rebuild.
	keepaliveTime := getEnvIntWithDefault("GRPC_KEEPALIVE_TIME_SEC", 0)
	keepaliveTimeout := getEnvIntWithDefault("GRPC_KEEPALIVE_TIMEOUT_SEC", 0)

	// Defaults mirror current hard-coded behavior
	handlersConcurrency := 8
	incomingBuffer := 100
	reconnectInterval := 5
	healthcheckInterval := 30
	pingInterval := 30

	return &Config{
		ServerName:                 serverName,
		CodexEnvPath:               codexEnvPath,
		CommunicationMode:          communicationMode,
		GrpcServerAddress:          endpoint.Address,
		ServerApiToken:             serverApiToken,
		IdentityTokenFile:          identityTokenFile,
		OrgID:                      orgID,
		GrpcUseTLS:                 endpoint.UseTLS,
		GrpcTLSInsecureSkipVerify:  insecureSkipVerify,
		HandlersConcurrency:        handlersConcurrency,
		IncomingEventsBuffer:       incomingBuffer,
		GrpcReconnectIntervalSec:   reconnectInterval,
		GrpcHealthcheckIntervalSec: healthcheckInterval,
		PingIntervalSec:            pingInterval,
		GrpcKeepaliveTimeSec:       keepaliveTime,
		GrpcKeepaliveTimeoutSec:    keepaliveTimeout,
		platformGrpcAddress:        platformGrpcAddress,
		platformGrpcUseTLS:         endpoint.PlatformUseTLS,
	}
}

// grpcUseTLS is the TLS choice the worker connects with: WithGrpcTLS or
// GRPC_USE_TLS, else the platform's DIBBLA_GRPC_USE_TLS while the address is
// still the platform's. nil = auto-detect based on address.
func (c *Config) grpcUseTLS() *bool {
	if c.GrpcUseTLS != nil {
		return c.GrpcUseTLS
	}
	if c.platformGrpcAddress != "" && c.GrpcServerAddress == c.platformGrpcAddress {
		return c.platformGrpcUseTLS
	}
	return nil
}

// chooseGrpcServerAddress records an address the user chose, which the
// platform's TLS hint does not describe.
func (c *Config) chooseGrpcServerAddress(address string) {
	c.GrpcServerAddress = address
	c.platformGrpcAddress = ""
	c.platformGrpcUseTLS = nil
}

// Configuration option functions

// WithServerName sets the server name
func WithServerName(name string) Option {
	return func(c *Config) { c.ServerName = name }
}

// Redis configuration removed (gRPC-only)

// WithWorkflowServer removed (unused in gRPC-only setup)

// Broadcast stream removed (gRPC-only)

// Communication mode is gRPC-only; kept for compatibility but forces grpc
func WithCommunicationMode(mode string) Option {
	return func(c *Config) { c.CommunicationMode = "grpc" }
}

// WithGrpcServerAddress sets the gRPC server address. It wins over
// GRPC_SERVER_ADDRESS and the platform's DIBBLA_GRPC_SERVER_ADDRESS, and the
// platform's TLS hint then no longer applies. A worker deployed on the Dibbla
// platform needs neither: set this only to reach a different server on
// purpose (local development: "localhost:50051").
func WithGrpcServerAddress(address string) Option {
	return func(c *Config) { c.chooseGrpcServerAddress(address) }
}

// WithServerApiToken sets the API token for authentication
func WithServerApiToken(token string) Option {
	return func(c *Config) { c.ServerApiToken = token }
}

// WithIdentityTokenFile points the SDK at a projected workload-identity
// token file (DIB-202). Rarely needed: the Dibbla platform injects
// DIBBLA_IDENTITY_TOKEN_FILE and the default mount path is probed
// automatically. An explicit ServerApiToken always wins over the file.
func WithIdentityTokenFile(path string) Option {
	return func(c *Config) { c.IdentityTokenFile = path }
}

// WithOrgID pins function registration to a specific organization. Use this
// when the API token's owner belongs to multiple orgs; the value is sent as
// x-org-id gRPC metadata and the platform verifies membership. When empty, the
// token's default organization is used.
func WithOrgID(orgID string) Option {
	return func(c *Config) { c.OrgID = orgID }
}

// WithGrpcTLS enables or disables TLS for gRPC connections. It wins over
// GRPC_USE_TLS and the platform's DIBBLA_GRPC_USE_TLS.
func WithGrpcTLS(useTLS bool) Option {
	return func(c *Config) { c.GrpcUseTLS = &useTLS }
}

// WithGrpcInsecureSkipVerify disables TLS certificate verification for gRPC
// connections. It only has an effect when TLS is in use (see WithGrpcTLS); the
// connection stays encrypted but the server certificate is not validated.
//
// This is insecure and exposes the connection to man-in-the-middle attacks.
// Use it only for self-signed or otherwise untrusted certificates in controlled
// environments (e.g. internal servers with a private CA). Prefer disabling TLS
// entirely with WithGrpcTLS(false) for plaintext local development.
func WithGrpcInsecureSkipVerify(skip bool) Option {
	return func(c *Config) { c.GrpcTLSInsecureSkipVerify = skip }
}

// WithGrpcMode configures the SDK to use gRPC communication
func WithGrpcMode(serverAddress string) Option {
	return func(c *Config) {
		c.CommunicationMode = "grpc"
		c.chooseGrpcServerAddress(serverAddress)
	}
}

// Redis mode removed

// Redis group removed

// WithCodexEnvPath sets the path to the codex environment file
func WithCodexEnvPath(path string) Option {
	return func(c *Config) { c.CodexEnvPath = path }
}

// WithPingInterval sets the interval for sending ping messages (in seconds)
// Set to 0 to disable ping messages
func WithPingInterval(seconds int) Option {
	return func(c *Config) { c.PingIntervalSec = seconds }
}

// WithGrpcKeepalive sets the HTTP/2 keepalive ping interval and ack timeout,
// in seconds. Zero values use the SDK defaults (300s / 20s). The keepalive
// detects silently-dead connections (dropped NAT/firewall mappings, VPN blips)
// so the worker reconnects on its own instead of hanging on a dead stream.
//
// Do not set timeSec below 300 when the worker dials a gRPC server directly:
// gRPC servers' default keepalive enforcement rejects faster pings with
// GOAWAY "too_many_pings". Behind an HTTP/2 proxy that answers pings itself
// (e.g. Traefik ingress), shorter intervals such as 30s are safe and give
// faster dead-connection detection.
func WithGrpcKeepalive(timeSec, timeoutSec int) Option {
	return func(c *Config) {
		c.GrpcKeepaliveTimeSec = timeSec
		c.GrpcKeepaliveTimeoutSec = timeoutSec
	}
}

// applyToEnvironment applies the configuration to environment variables
// This ensures compatibility with existing code that reads from env vars
func (c *Config) applyToEnvironment() {
	os.Setenv("SERVER_NAME", c.ServerName)
	if c.CodexEnvPath != "" {
		os.Setenv("CODEX_ENV_PATH", c.CodexEnvPath)
	}
}

// getEnvWithDefault returns the environment variable value or a default if not set
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntWithDefault returns the environment variable parsed as an int, or
// the default if unset or not a valid integer.
func getEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	return defaultValue
}
