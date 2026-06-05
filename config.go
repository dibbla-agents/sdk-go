package sdk

import (
	"os"
)

// Config holds the configuration for the SDK server
type Config struct {
	ServerName   string
	CodexEnvPath string

	// Communication configuration
	CommunicationMode string // will be forced to "grpc"
	GrpcServerAddress string // Address of the gRPC workflow server
	ServerApiToken    string // API token for authentication
	OrgID             string // optional: pin registration to a specific org (multi-org users)
	GrpcUseTLS        *bool  // nil = auto-detect based on address
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
}

// Option is a functional option for configuring the SDK
type Option func(*Config)

// defaultConfig returns the default configuration, reading from environment variables
func defaultConfig() *Config {
	serverName := getEnvWithDefault("SERVER_NAME", "codex-go-worker")
	codexEnvPath := getEnvWithDefault("CODEX_ENV_PATH", "")
	communicationMode := getEnvWithDefault("COMMUNICATION_MODE", "grpc")
	grpcServerAddress := getEnvWithDefault("GRPC_SERVER_ADDRESS", "grpc.dibbla.com:443")
	serverApiToken := getEnvWithDefault("SERVER_API_TOKEN", "")
	orgID := getEnvWithDefault("SERVER_ORG_ID", "")
	
	// TLS configuration with auto-detection
	var defaultTLS *bool = nil
	if tlsEnv := os.Getenv("GRPC_USE_TLS"); tlsEnv != "" {
		val := (tlsEnv == "true" || tlsEnv == "1")
		defaultTLS = &val
	}

	// TLS certificate verification can be skipped via env (insecure)
	insecureSkipVerify := false
	if v := os.Getenv("GRPC_TLS_INSECURE_SKIP_VERIFY"); v == "true" || v == "1" {
		insecureSkipVerify = true
	}
	
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
		GrpcServerAddress:          grpcServerAddress,
		ServerApiToken:             serverApiToken,
		OrgID:                      orgID,
		GrpcUseTLS:                 defaultTLS,
		GrpcTLSInsecureSkipVerify:  insecureSkipVerify,
		HandlersConcurrency:        handlersConcurrency,
		IncomingEventsBuffer:       incomingBuffer,
		GrpcReconnectIntervalSec:   reconnectInterval,
		GrpcHealthcheckIntervalSec: healthcheckInterval,
		PingIntervalSec:            pingInterval,
	}
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

// WithGrpcServerAddress sets the gRPC server address
func WithGrpcServerAddress(address string) Option {
	return func(c *Config) { c.GrpcServerAddress = address }
}

// WithServerApiToken sets the API token for authentication
func WithServerApiToken(token string) Option {
	return func(c *Config) { c.ServerApiToken = token }
}

// WithOrgID pins function registration to a specific organization. Use this
// when the API token's owner belongs to multiple orgs; the value is sent as
// x-org-id gRPC metadata and the platform verifies membership. When empty, the
// token's default organization is used.
func WithOrgID(orgID string) Option {
	return func(c *Config) { c.OrgID = orgID }
}

// WithGrpcTLS enables or disables TLS for gRPC connections
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
		c.GrpcServerAddress = serverAddress
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

