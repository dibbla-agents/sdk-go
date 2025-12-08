package jobs

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/dibbla-agents/sdk-go/internal/state"
	"github.com/dibbla-agents/sdk-go/internal/types"
)

// JobHost manages job registration and execution via gRPC
type JobHost struct {
	globalState *state.GlobalState
	registry    *JobRegistry
	hostID      string
	serverName  string

	mu      sync.RWMutex
	started bool
}

// NewJobHost creates a new JobHost using the SDK's GlobalState
// The hostID identifies this job host to the server
func NewJobHost(gs *state.GlobalState, hostID string) *JobHost {
	return &JobHost{
		globalState: gs,
		registry:    NewJobRegistry(),
		hostID:      hostID,
		serverName:  gs.ServerName,
	}
}

// RegisterJob adds a job handler to this host
func (h *JobHost) RegisterJob(handler JobHandler) error {
	return h.registry.Register(handler)
}

// Start initializes the job host:
// 1. Registers the job_trigger handler with the dispatcher
// 2. Sends job registration to the server
func (h *JobHost) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.started {
		return nil
	}

	// Validate we have jobs registered
	if len(h.registry.List()) == 0 {
		log.Println("Warning: JobHost started with no jobs registered")
	}

	// Register handler for job triggers from server
	h.globalState.Dispatcher.Register(types.EventJobTrigger, h.handleJobTrigger)

	// Send job registration to server
	h.sendRegistration()

	h.started = true
	log.Printf("JobHost started: host_id=%s, jobs=%v", h.hostID, h.registry.List())

	return nil
}

// handleJobTrigger processes incoming job_trigger events from the server
func (h *JobHost) handleJobTrigger(msg *types.EventMessage) {
	meta := ParseJobEventMeta(msg.Meta)

	log.Printf("Received job trigger: job_id=%s, run_id=%s", meta.JobID, msg.Run)

	// Parse job arguments from payload
	var args map[string]interface{}
	if msg.Payload != nil && len(*msg.Payload) > 0 {
		if err := json.Unmarshal(*msg.Payload, &args); err != nil {
			log.Printf("Failed to parse job arguments: %v", err)
			h.sendJobFailed(msg.Run, meta.JobID, meta.JobName, err)
			return
		}
	}

	// Execute job asynchronously
	go h.executeJob(msg.Run, meta.JobID, meta.JobName, args)
}

// executeJob runs a job and handles lifecycle events
func (h *JobHost) executeJob(runID, jobID, jobName string, args map[string]interface{}) {
	// Get handler from registry
	handler, err := h.registry.Get(jobID)
	if err != nil {
		log.Printf("Job not found: %s", jobID)
		h.sendJobFailed(runID, jobID, jobName, err)
		return
	}

	// Use handler's job name if not provided
	if jobName == "" {
		jobName = handler.GetJobName()
	}

	// Create job context with logger
	logger := h.createLogger(runID, jobID, jobName)
	ctx := &JobContext{
		RunID:   runID,
		JobID:   jobID,
		JobName: jobName,
		Args:    args,
		Logger:  logger,
	}

	// Send job_started event
	h.sendJobStarted(runID, jobID, jobName)

	// Execute the job
	if err := handler.Execute(ctx); err != nil {
		log.Printf("Job failed: job_id=%s, run_id=%s, error=%v", jobID, runID, err)
		h.sendJobFailed(runID, jobID, jobName, err)
		return
	}

	// Send job_completed event
	h.sendJobCompleted(runID, jobID, jobName)
	log.Printf("Job completed: job_id=%s, run_id=%s", jobID, runID)
}

// sendRegistration sends job_registration event to server
func (h *JobHost) sendRegistration() {
	// Build job schemas
	jobSchemas := make(map[string]interface{})
	for _, handler := range h.registry.GetAllHandlers() {
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
	}

	meta := map[string]any{
		"host_id": h.hostID,
		"jobs":    jobSchemas,
	}

	event := &types.EventMessage{
		Server: h.serverName,
		Event:  types.EventJobRegistration,
		Meta:   &meta,
	}

	if err := h.globalState.WorkflowComm.SendEvent(event); err != nil {
		log.Printf("Failed to send job registration: %v", err)
	} else {
		log.Printf("Sent job registration: host_id=%s, jobs=%d", h.hostID, len(jobSchemas))
	}
}

// sendJobStarted sends job_started event
func (h *JobHost) sendJobStarted(runID, jobID, jobName string) {
	meta := NewJobEventMeta(jobID, jobName)
	meta.Status = string(StatusInProgress)
	metaMap := meta.ToMap()

	event := &types.EventMessage{
		Server:        h.serverName,
		Event:         types.EventJobStarted,
		Run:           runID,
		CorrelationID: runID,
		Meta:          &metaMap,
	}

	h.globalState.WorkflowComm.SendEvent(event)
}

// sendJobCompleted sends job_completed event
func (h *JobHost) sendJobCompleted(runID, jobID, jobName string) {
	meta := NewJobEventMeta(jobID, jobName)
	meta.Status = string(StatusCompleted)
	metaMap := meta.ToMap()

	event := &types.EventMessage{
		Server:        h.serverName,
		Event:         types.EventJobCompleted,
		Run:           runID,
		CorrelationID: runID,
		Meta:          &metaMap,
	}

	h.globalState.WorkflowComm.SendEvent(event)
}

// sendJobFailed sends job_failed event
func (h *JobHost) sendJobFailed(runID, jobID, jobName string, err error) {
	meta := NewJobEventMeta(jobID, jobName)
	meta.Status = string(StatusFailed)
	meta.Error = err.Error()
	metaMap := meta.ToMap()

	event := &types.EventMessage{
		Server:        h.serverName,
		Event:         types.EventJobFailed,
		Run:           runID,
		CorrelationID: runID,
		Meta:          &metaMap,
	}

	h.globalState.WorkflowComm.SendEvent(event)
}

// createLogger creates a Logger for a specific job run
func (h *JobHost) createLogger(runID, jobID, jobName string) *Logger {
	return NewLogger(h.globalState.WorkflowComm, h.serverName, runID, jobID, jobName)
}

// GetRegistry returns the job registry (for testing/inspection)
func (h *JobHost) GetRegistry() *JobRegistry {
	return h.registry
}

// IsStarted returns whether the job host has been started
func (h *JobHost) IsStarted() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.started
}
