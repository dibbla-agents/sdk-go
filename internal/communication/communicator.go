package communication

import (
	"time"

	"github.com/dibbla-agents/sdk-go/internal/types"
)

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

	// WaitForConnection blocks until the connection is established or timeout occurs
	WaitForConnection(timeout time.Duration) error

	// SetOnReconnect sets a callback that will be called when the connection is
	// re-established after a disconnect
	SetOnReconnect(callback func())
}
