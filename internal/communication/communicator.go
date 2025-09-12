package communication

import "github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"

// WorkflowCommunicator defines the interface for communicating with the workflow server
type WorkflowCommunicator interface {
	// SendEvent sends an event to the workflow server
	SendEvent(event *types.EventMessage) error

	// ReceiveEvents returns a channel for receiving events from the workflow server
	ReceiveEvents() <-chan *types.EventMessage

	// Close closes the communicator and cleans up resources
	Close() error

	// IsConnected returns whether the communicator is currently connected
	IsConnected() bool
}

