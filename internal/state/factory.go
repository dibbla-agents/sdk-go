package state

import (
	"fmt"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/basefunction"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/communication"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/grpccache"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/grpcstore"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/maps"
	"os"
)

// CommunicationConfig holds configuration for communication setup
type CommunicationConfig struct {
	ServerName             string
	GrpcServerAddress      string
	ServerApiToken         string
	IncomingBuffer         int
	ReconnectIntervalSec   int
	HealthcheckIntervalSec int
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

	// Initialize gRPC cache client when a communicator exists
	if gs.WorkflowComm != nil {
		gs.GrpcCache = grpccache.NewClient(gs.WorkflowComm, config.ServerName)
		gs.GrpcStore = grpcstore.NewClient(gs.WorkflowComm, config.ServerName)
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
	grpcCommunicator := communication.NewGrpcCommunicatorWithOptions(
		config.GrpcServerAddress,
		config.ServerName,
		config.ServerApiToken,
		incoming,
		reconn,
		health,
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
		GrpcServerAddress: getEnvWithDefault("GRPC_SERVER_ADDRESS", "localhost:9090"),
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
