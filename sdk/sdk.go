package sdk

import (
	"fmt"
	"log"

	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/basefunction"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/dispatcher"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/handlers"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/rpc"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/state"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"

	"github.com/joho/godotenv"
)

// Server represents the SDK server instance
type Server struct {
	config      *Config
	globalState *state.GlobalState
	functions   []FunctionBuilder
}

// New creates a new SDK server instance with the provided options
func New(opts ...Option) (*Server, error) {
	config := defaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	// Apply configuration to environment variables for compatibility
	config.applyToEnvironment()

	server := &Server{
		config:    config,
		functions: make([]FunctionBuilder, 0),
	}

	return server, nil
}

// RegisterFunction adds a function to the server
func (s *Server) RegisterFunction(fn FunctionBuilder) {
	s.functions = append(s.functions, fn)
}

// initializeEnvironment loads environment files as needed
func (s *Server) initializeEnvironment() error {
	// Load main .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Error loading .env file, assuming Docker environment")
	}

	// Load codex .env file if specified
	if s.config.CodexEnvPath != "" {
		if err := godotenv.Load(s.config.CodexEnvPath); err != nil {
			log.Println("Error loading codex .env file, assuming Docker environment")
		}
	} else {
		log.Println("CODEX_ENV_PATH is not set, assuming Docker environment")
	}

	return nil
}

// initializeGlobalState creates and initializes the global state
func (s *Server) initializeGlobalState() error {
	// Create communication config from SDK config
	commConfig := state.CommunicationConfig{
		ServerName:             s.config.ServerName,
		GrpcServerAddress:      s.config.GrpcServerAddress,
		ServerApiToken:         s.config.ServerApiToken,
		IncomingBuffer:         s.config.IncomingEventsBuffer,
		ReconnectIntervalSec:   s.config.GrpcReconnectIntervalSec,
		HealthcheckIntervalSec: s.config.GrpcHealthcheckIntervalSec,
	}

	globalState, err := state.NewGlobalStateWithMode(commConfig)
	if err != nil {
		return fmt.Errorf("failed to create global state: %w", err)
	}

	// No Redis-based function cache to flush; gRPC cache is used implicitly

	// Set up RPC client after global state is created to avoid import cycles
	s.globalState = globalState
	s.globalState.RpcClient = rpc.NewRpcClientWithCommunicator(globalState.WorkflowComm)

	// Initialize dispatcher with configured buffer and concurrency
	s.globalState.Dispatcher = dispatcher.NewDispatcher(s.config.IncomingEventsBuffer)
	s.globalState.Dispatcher.Start(s.config.HandlersConcurrency)

	// GrpcCache client is created in state.NewGlobalStateWithMode when a communicator exists
	// Nothing more to do here

	log.Printf("Initialized global state (gRPC mode)")
	return nil
}

// registerAndPublishFunctions registers all functions with the global state
func (s *Server) registerAndPublishFunctions() {
	functionMap := map[string]basefunction.FunctionInterface{}

	// Build and register each function
	for _, fnBuilder := range s.functions {
		function := fnBuilder.Build(s.globalState)
		s.registerFunction(functionMap, function)
	}

	// No Redis publishing in gRPC-only setup

	// Store functions in global state
	for key, function := range functionMap {
		s.globalState.Functions.Store(key, function)
	}

	log.Printf("Registered %d functions", len(functionMap))
}

// registerFunction is a helper that initializes a function, sets its cache, and adds it to the function map
func (s *Server) registerFunction(functionMap map[string]basefunction.FunctionInterface, function basefunction.FunctionInterface) {
	// Inject server name into function definition if supported
	switch fn := function.(type) {
	case interface{ SetServer(string) }:
		fn.SetServer(s.globalState.ServerName)
	}
	s.setFunctionCache(function)
	redisKey := s.getRedisKey(function)
	functionMap[redisKey] = function
}

// getRedisKey generates the Redis key for a function
func (s *Server) getRedisKey(function basefunction.FunctionInterface) string {
	return types.FunctionKey(s.globalState.ServerName, function.GetName(), function.GetVersion())
}

// Redis publishing removed in gRPC-only setup

// setFunctionCache sets the cache for a function, preferring gRPC cache and falling back to Redis
func (s *Server) setFunctionCache(function basefunction.FunctionInterface) {
	switch fn := function.(type) {
	case interface {
		SetCache(basefunction.FunctionCache)
	}:
		// Use gRPC cache when available
		if s.globalState != nil && s.globalState.GrpcCache != nil {
			fn.SetCache(s.globalState.GrpcCache)
			return
		}
	}
}

// registerServer registers the server with the workflow server
func (s *Server) registerServer() {
	// Create an empty event state for server registration
	eventState := &state.EventState{
		Function:       "",
		Version:        "",
		Node:           "",
		Workflow:       "",
		Server:         s.globalState.ServerName,
		FunctionServer: s.globalState.ServerName,
		CorrelationID:  "startup",
	}

	// Send server name and function list
	handlers.HandleListFunctions(s.globalState, eventState)
	log.Printf("Server '%s' registered with workflow server", s.globalState.ServerName)
}

// sendStartupBroadcast sends the function list on startup
func (s *Server) sendStartupBroadcast() {
	startupEventState := state.NewEventState(
		s.globalState.ServerName, // server
		"startup",                // function
		"1.0",                    // version
		"startup",                // node
		"startup",                // workflow
		"startup",                // run
		s.globalState.ServerName, // functionServer
		"startup",                // correlationID
	)
	handlers.HandleListFunctions(s.globalState, startupEventState)
	log.Println("Startup function list broadcast sent")
}

// Start initializes and starts the server
func (s *Server) Start() error {
	log.Printf("Starting server with name: %s", s.config.ServerName)

	// Initialize environment
	if err := s.initializeEnvironment(); err != nil {
		return fmt.Errorf("failed to initialize environment: %w", err)
	}

	// Initialize global state
	if err := s.initializeGlobalState(); err != nil {
		return fmt.Errorf("failed to initialize global state: %w", err)
	}

	// Register and publish functions
	s.registerAndPublishFunctions()

	// Register server with workflow server
	s.registerServer()

	// Send startup broadcast
	s.sendStartupBroadcast()

	// Activate handlers
	handlers.Activate(s.globalState)
	log.Println("Stream listeners activated, server running...")

	// Block forever
	select {}
}

// GetGlobalState returns the global state for advanced use cases
func (s *Server) GetGlobalState() *state.GlobalState {
	return s.globalState
}

// GetConfig returns the current configuration
func (s *Server) GetConfig() *Config {
	return s.config
}
