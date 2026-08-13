package sdk

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/dibbla-agents/sdk-go/internal/basefunction"
	"github.com/dibbla-agents/sdk-go/internal/diagnostics"
	"github.com/dibbla-agents/sdk-go/internal/dispatcher"
	"github.com/dibbla-agents/sdk-go/internal/handlers"
	"github.com/dibbla-agents/sdk-go/internal/rpc"
	"github.com/dibbla-agents/sdk-go/internal/state"
	"github.com/dibbla-agents/sdk-go/internal/types"
	"github.com/dibbla-agents/sdk-go/jobs"

	"github.com/joho/godotenv"
)

// Server represents the SDK server instance
type Server struct {
	config      *Config
	globalState *state.GlobalState
	functions   []FunctionBuilder
	jobs        []jobs.JobHandler
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
		jobs:      make([]jobs.JobHandler, 0),
	}

	return server, nil
}

// RegisterFunction adds a function to the server
func (s *Server) RegisterFunction(fn FunctionBuilder) {
	s.functions = append(s.functions, fn)
}

// RegisterJob adds a job handler to the server.
// Jobs will be registered with the workflow server when Start() is called.
func (s *Server) RegisterJob(handler jobs.JobHandler) {
	s.jobs = append(s.jobs, handler)
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
		IdentityTokenFile:      s.config.IdentityTokenFile,
		OrgID:                  s.config.OrgID,
		IncomingBuffer:         s.config.IncomingEventsBuffer,
		ReconnectIntervalSec:   s.config.GrpcReconnectIntervalSec,
		HealthcheckIntervalSec: s.config.GrpcHealthcheckIntervalSec,
		PingIntervalSec:        s.config.PingIntervalSec,
		UseTLS:                 s.config.GrpcUseTLS,
		TLSInsecureSkipVerify:  s.config.GrpcTLSInsecureSkipVerify,
		KeepaliveTimeSec:       s.config.GrpcKeepaliveTimeSec,
		KeepaliveTimeoutSec:    s.config.GrpcKeepaliveTimeoutSec,
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

// Init initializes the server's global state and gRPC connections.
// This is called automatically by Start(), but can be called explicitly
// if you need access to GlobalState before starting the server.
// Multiple calls to Init() are safe - subsequent calls are no-ops.
func (s *Server) Init() error {
	if s.globalState != nil {
		return nil // Already initialized
	}

	log.Printf("Initializing server: %s", s.config.ServerName)

	// Initialize environment
	if err := s.initializeEnvironment(); err != nil {
		return fmt.Errorf("failed to initialize environment: %w", err)
	}

	// Initialize global state
	if err := s.initializeGlobalState(); err != nil {
		return fmt.Errorf("failed to initialize global state: %w", err)
	}

	return nil
}

// Start initializes and starts the server
func (s *Server) Start() error {
	log.Printf("Starting server with name: %s", s.config.ServerName)

	// Optional pprof endpoint (SDK_PPROF_ADDR) and GOMEMLIMIT from the cgroup
	// memory limit. Best-effort; never fails startup.
	diagnostics.Setup()

	// Initialize if not already done
	if err := s.Init(); err != nil {
		return err
	}

	// Wait for gRPC connection before sending any registrations
	log.Println("Waiting for gRPC connection...")
	if err := s.globalState.WorkflowComm.WaitForConnection(30 * time.Second); err != nil {
		return fmt.Errorf("failed to establish connection: %w", err)
	}

	// Setup reconnect callback to re-register everything on reconnect
	s.globalState.WorkflowComm.SetOnReconnect(s.onReconnect)

	// Register and publish functions
	s.registerAndPublishFunctions()

	// Register server with workflow server
	s.registerServer()

	// Send startup broadcast
	s.sendStartupBroadcast()

	// Register jobs if any
	if len(s.jobs) > 0 {
		s.registerJobTriggerHandler()
		s.registerJobs()
	}

	// Activate handlers
	handlers.Activate(s.globalState)
	log.Println("Stream listeners activated, server running...")

	// Block forever
	select {}
}

// registerJobs sends job registration event to the workflow server
func (s *Server) registerJobs() {
	if len(s.jobs) == 0 {
		return
	}

	// Build job schemas from registered jobs
	jobSchemas := make(map[string]interface{})
	jobIDs := make([]string, 0, len(s.jobs))

	for _, handler := range s.jobs {
		params := handler.GetParameters()
		paramMap := make(map[string]interface{})
		for _, p := range params {
			paramMap[p.Name] = map[string]interface{}{
				"type":     p.Type,
				"required": p.Required,
				"default":  p.Default,
			}
		}

		jobSchemas[handler.GetJobID()] = map[string]interface{}{
			"name":       handler.GetJobName(),
			"parameters": paramMap,
		}
		jobIDs = append(jobIDs, handler.GetJobID())
	}

	meta := map[string]any{
		"host_id": s.config.ServerName,
		"jobs":    jobSchemas,
	}

	event := &types.EventMessage{
		Server: s.globalState.ServerName,
		Event:  types.EventJobRegistration,
		Meta:   &meta,
	}

	if err := s.globalState.WorkflowComm.SendEvent(event); err != nil {
		log.Printf("Failed to send job registration: %v", err)
	} else {
		log.Printf("Registered %d jobs: %v", len(s.jobs), jobIDs)
	}
}

// registerJobTriggerHandler registers the handler for incoming job triggers.
// Registered direct: the handler only parses metadata and spawns the job
// goroutine, so it never blocks, and triggers must not queue behind (or be
// dropped by) a saturated function-request pool.
func (s *Server) registerJobTriggerHandler() {
	s.globalState.Dispatcher.RegisterDirect(types.EventJobTrigger, s.handleJobTrigger)
}

// handleJobTrigger processes incoming job_trigger events from the server
func (s *Server) handleJobTrigger(msg *types.EventMessage) {
	meta := jobs.ParseJobEventMeta(msg.Meta)

	log.Printf("Received job trigger: job_id=%s, run_id=%s", meta.JobID, msg.Run)

	// Parse job arguments from payload
	var args map[string]interface{}
	if msg.Payload != nil && len(*msg.Payload) > 0 {
		if err := json.Unmarshal(*msg.Payload, &args); err != nil {
			log.Printf("Failed to parse job arguments: %v", err)
			s.sendJobFailed(msg.Run, meta.JobID, meta.JobName, err)
			return
		}
	}

	// Execute job asynchronously
	go s.executeJob(msg.Run, meta.JobID, meta.JobName, args)
}

// executeJob runs a job and handles lifecycle events
func (s *Server) executeJob(runID, jobID, jobName string, args map[string]interface{}) {
	// Find the job handler
	handler := s.getJobHandler(jobID)
	if handler == nil {
		err := fmt.Errorf("job not found: %s", jobID)
		log.Printf("Job not found: %s", jobID)
		s.sendJobFailed(runID, jobID, jobName, err)
		return
	}

	// Use handler's job name if not provided
	if jobName == "" {
		jobName = handler.GetJobName()
	}

	// Create job context with logger
	logger := s.createJobLogger(runID, jobID, jobName)
	ctx := &jobs.JobContext{
		RunID:   runID,
		JobID:   jobID,
		JobName: jobName,
		Args:    args,
		Logger:  logger,
	}

	// Send job_started event
	s.sendJobStarted(runID, jobID, jobName)

	// Execute the job
	if err := handler.Execute(ctx); err != nil {
		log.Printf("Job failed: job_id=%s, run_id=%s, error=%v", jobID, runID, err)
		s.sendJobFailed(runID, jobID, jobName, err)
		return
	}

	// Send job_completed event
	s.sendJobCompleted(runID, jobID, jobName)
	log.Printf("Job completed: job_id=%s, run_id=%s", jobID, runID)
}

// getJobHandler finds a job handler by its ID
func (s *Server) getJobHandler(jobID string) jobs.JobHandler {
	for _, handler := range s.jobs {
		if handler.GetJobID() == jobID {
			return handler
		}
	}
	return nil
}

// createJobLogger creates a Logger for a specific job run
func (s *Server) createJobLogger(runID, jobID, jobName string) *jobs.Logger {
	return jobs.NewLogger(s.globalState.WorkflowComm, s.globalState.ServerName, runID, jobID, jobName)
}

// sendJobStarted sends job_started event
func (s *Server) sendJobStarted(runID, jobID, jobName string) {
	meta := jobs.NewJobEventMeta(jobID, jobName)
	meta.Status = string(jobs.StatusInProgress)
	metaMap := meta.ToMap()

	event := &types.EventMessage{
		Server:        s.globalState.ServerName,
		Event:         types.EventJobStarted,
		Run:           runID,
		CorrelationID: runID,
		Meta:          &metaMap,
	}

	if err := s.globalState.WorkflowComm.SendEvent(event); err != nil {
		log.Printf("Failed to send job_started event: %v", err)
	}
}

// sendJobCompleted sends job_completed event
func (s *Server) sendJobCompleted(runID, jobID, jobName string) {
	meta := jobs.NewJobEventMeta(jobID, jobName)
	meta.Status = string(jobs.StatusCompleted)
	metaMap := meta.ToMap()

	event := &types.EventMessage{
		Server:        s.globalState.ServerName,
		Event:         types.EventJobCompleted,
		Run:           runID,
		CorrelationID: runID,
		Meta:          &metaMap,
	}

	if err := s.globalState.WorkflowComm.SendEvent(event); err != nil {
		log.Printf("Failed to send job_completed event: %v", err)
	}
}

// sendJobFailed sends job_failed event
func (s *Server) sendJobFailed(runID, jobID, jobName string, err error) {
	meta := jobs.NewJobEventMeta(jobID, jobName)
	meta.Status = string(jobs.StatusFailed)
	meta.Error = err.Error()
	metaMap := meta.ToMap()

	event := &types.EventMessage{
		Server:        s.globalState.ServerName,
		Event:         types.EventJobFailed,
		Run:           runID,
		CorrelationID: runID,
		Meta:          &metaMap,
	}

	if err := s.globalState.WorkflowComm.SendEvent(event); err != nil {
		log.Printf("Failed to send job_failed event: %v", err)
	}
}

// onReconnect is called when the gRPC connection is re-established after a disconnect.
// It re-registers all functions and jobs with the workflow server.
func (s *Server) onReconnect() {
	log.Println("Connection re-established, re-registering with workflow server...")

	// Re-register server and functions
	s.registerServer()
	s.sendStartupBroadcast()

	// Re-register jobs if any
	if len(s.jobs) > 0 {
		s.registerJobs()
	}

	log.Println("Re-registration complete")
}

// GetGlobalState returns the global state for advanced use cases
func (s *Server) GetGlobalState() *state.GlobalState {
	return s.globalState
}

// GetConfig returns the current configuration
func (s *Server) GetConfig() *Config {
	return s.config
}
