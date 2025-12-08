package jobs

import (
	"fmt"
	"sync"
)

// JobRegistry manages registered job handlers
type JobRegistry struct {
	mu   sync.RWMutex
	jobs map[string]JobHandler
}

// NewJobRegistry creates a new job registry
func NewJobRegistry() *JobRegistry {
	return &JobRegistry{
		jobs: make(map[string]JobHandler),
	}
}

// Register adds a job handler to the registry
func (r *JobRegistry) Register(handler JobHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	jobID := handler.GetJobID()
	if jobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	if _, exists := r.jobs[jobID]; exists {
		return fmt.Errorf("job with ID %s is already registered", jobID)
	}

	r.jobs[jobID] = handler
	return nil
}

// Get retrieves a job handler by ID
func (r *JobRegistry) Get(jobID string) (JobHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job with ID %s not found", jobID)
	}

	return handler, nil
}

// List returns all registered job IDs
func (r *JobRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	jobIDs := make([]string, 0, len(r.jobs))
	for jobID := range r.jobs {
		jobIDs = append(jobIDs, jobID)
	}
	return jobIDs
}

// GetAllHandlers returns all registered handlers
func (r *JobRegistry) GetAllHandlers() []JobHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := make([]JobHandler, 0, len(r.jobs))
	for _, handler := range r.jobs {
		handlers = append(handlers, handler)
	}
	return handlers
}

// Count returns the number of registered jobs
func (r *JobRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.jobs)
}
