package state

import (
	"fmt"
	"os"

	"github.com/dibbla-agents/sdk-go/internal/basefunction"
	"github.com/dibbla-agents/sdk-go/internal/communication"
	"github.com/dibbla-agents/sdk-go/internal/grpccache"
	"github.com/dibbla-agents/sdk-go/internal/grpcstore"
	"github.com/dibbla-agents/sdk-go/internal/maps"
	"github.com/dibbla-agents/sdk-go/internal/oauth"
)

// CommunicationConfig holds configuration for communication setup
type CommunicationConfig struct {
	ServerName             string
	GrpcServerAddress      string
	ServerApiToken         string
	IncomingBuffer         int
	ReconnectIntervalSec   int
	HealthcheckIntervalSec int
	PingIntervalSec        int   // Interval for sending ping messages (0 = disabled)
	UseTLS                 *bool // nil = auto-detect
}

// NewGlobalStateWithMode creates a GlobalState with the specified communication mode
func NewGlobalStateWithMode(config CommunicationConfig) (*GlobalState, error) {
	gs := &GlobalState{
		ServerName:       config.ServerName,
		Functions:        maps.NewSafeFunctionMap[string, basefunction.FunctionInterface](),
		ResponseHandlers: maps.NewSafeFunctionMap[string, chan *[]byte](),
		ExecutionState:   maps.NewSafeFunctionMap[string, any](),
	}

	var err error
	var workflowComm communication.WorkflowCommunicator

	// gRPC is the only supported communication mode
	workflowComm, err = setupGrpcMode(gs, config)

	if err != nil {
		return nil, fmt.Errorf("failed to setup gRPC communication: %w", err)
	}

	gs.WorkflowComm = workflowComm

	// RPC client will be set up separately to avoid import cycles
	gs.RpcClient = nil

	// Initialize gRPC clients when a communicator exists
	if gs.WorkflowComm != nil {
		gs.GrpcCache = grpccache.NewClient(gs.WorkflowComm, config.ServerName)
		gs.GrpcStore = grpcstore.NewClient(gs.WorkflowComm, config.ServerName)
		gs.OAuth = oauth.NewClient(gs.WorkflowComm, config.ServerName)
	}

	return gs, nil
}

// setupGrpcMode initializes gRPC-based communication
func setupGrpcMode(gs *GlobalState, config CommunicationConfig) (communication.WorkflowCommunicator, error) {
	// Create gRPC communicator with provided options or safe defaults
	incoming := config.IncomingBuffer
	if incoming <= 0 {
		incoming = 100
	}
	reconn := config.ReconnectIntervalSec
	if reconn <= 0 {
		reconn = 5
	}
	health := config.HealthcheckIntervalSec
	if health <= 0 {
		health = 30
	}
	ping := config.PingIntervalSec
	if ping < 0 {
		ping = 0 // 0 means disabled
	}

	// Determine TLS setting
	// Default to auto-detection based on address (TLS for production, no TLS for localhost)
	useTLS := true
	if config.UseTLS != nil {
		// Explicit override takes precedence
		useTLS = *config.UseTLS
	} else {
		// Auto-detect: TLS for production addresses, no TLS for localhost
		useTLS = communication.ShouldUseTLS(config.GrpcServerAddress)
	}

	grpcCommunicator := communication.NewGrpcCommunicatorWithTLS(
		config.GrpcServerAddress,
		config.ServerName,
		config.ServerApiToken,
		incoming,
		reconn,
		health,
		ping,
		useTLS,
	)

	// Connect to gRPC server
	if err := grpcCommunicator.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	return grpcCommunicator, nil
}

// NewGlobalStateFromEnvironment creates a GlobalState using environment variables
func NewGlobalStateFromEnvironment() (*GlobalState, error) {
	config := CommunicationConfig{
		ServerName:        getEnvWithDefault("SERVER_NAME", "go-toolserver"),
		GrpcServerAddress: getEnvWithDefault("GRPC_SERVER_ADDRESS", "grpc.dibbla.com:443"),
	}
	return NewGlobalStateWithMode(config)
}

// getEnvWithDefault returns environment variable value or default if not set
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// NewGlobalState creates a GlobalState using the legacy Redis-only approach
// This maintains backward compatibility
func NewGlobalState() *GlobalState { gs, _ := NewGlobalStateFromEnvironment(); return gs }
