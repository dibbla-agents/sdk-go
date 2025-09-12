package grpcstore

import (
	"context"
	"fmt"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/communication"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/correlation"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/utils"
	"time"
)

// Client provides a store client over the workflow gRPC stream using correlation IDs
type Client struct {
	communicator   communication.WorkflowCommunicator
	router         *correlation.Router
	defaultTimeout time.Duration
	serverName     string
}

// NewClient creates a new gRPC-based store client
func NewClient(comm communication.WorkflowCommunicator, serverName string) *Client {
	return &Client{
		communicator:   comm,
		router:         correlation.NewRouter(),
		defaultTimeout: 30 * time.Second,
		serverName:     serverName,
	}
}

// Get requests a stored value by workflow and key and waits for a store_get_response with the same correlation ID
// The response value is returned in the payload
func (c *Client) Get(ctx context.Context, workflowID, key string) ([]byte, error) {
	if c == nil || c.communicator == nil {
		return nil, fmt.Errorf("grpcstore: no communicator configured")
	}

	correlationID := utils.UID()
	responseChan := c.router.Register(correlationID, 1)
	defer c.router.Remove(correlationID)

	meta := map[string]any{
		"Workflow":       workflowID,
		"Key":            key,
		"calling_server": c.serverName,
	}

	event := types.EventMessage{
		Workflow:      workflowID,
		Event:         types.EventStoreGetRequest,
		Text:          "Store get request",
		Meta:          &meta,
		Payload:       nil,
		CorrelationID: correlationID,
	}

	if err := c.communicator.SendEvent(&event); err != nil {
		return nil, fmt.Errorf("grpcstore: failed to send store_get_request: %w", err)
	}

	select {
	case resp := <-responseChan:
		if resp.Payload == nil {
			return nil, fmt.Errorf("grpcstore: empty store_get_response payload")
		}
		return *resp.Payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetString is a convenience to return string data
func (c *Client) GetString(ctx context.Context, workflowID, key string) (string, error) {
	b, err := c.Get(ctx, workflowID, key)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// GetWithTimeout uses the client's default timeout
func (c *Client) GetWithTimeout(workflowID, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.defaultTimeout)
	defer cancel()
	return c.Get(ctx, workflowID, key)
}

// Set stores a value for a workflow/key. This is fire-and-forget by default.
func (c *Client) Set(ctx context.Context, workflowID, key string, value []byte) error {
	if c == nil || c.communicator == nil {
		return fmt.Errorf("grpcstore: no communicator configured")
	}

	payloadCopy := make([]byte, len(value))
	copy(payloadCopy, value)

	meta := map[string]any{
		"Workflow":       workflowID,
		"Key":            key,
		"calling_server": c.serverName,
	}

	event := types.EventMessage{
		Workflow:      workflowID,
		Event:         types.EventStoreSetRequest,
		Text:          "Store set request",
		Meta:          &meta,
		Payload:       &payloadCopy,
		CorrelationID: utils.UID(),
	}

	if err := c.communicator.SendEvent(&event); err != nil {
		return fmt.Errorf("grpcstore: failed to send store_set_request: %w", err)
	}
	return nil
}

// SetString is a convenience to send string data
func (c *Client) SetString(ctx context.Context, workflowID, key, value string) error {
	return c.Set(ctx, workflowID, key, []byte(value))
}

// SetWithTimeout uses the client's default timeout (fire-and-forget)
func (c *Client) SetWithTimeout(workflowID, key string, value []byte) error {
	return c.Set(context.Background(), workflowID, key, value)
}

// HandleResponse delivers store response events to the waiting goroutine using the correlation ID
// Supported events: store_get_response (and optionally store_set_response if server sends it)
func (c *Client) HandleResponse(response types.EventMessage) {
	if c == nil {
		return
	}
	if response.Event != types.EventStoreGetResponse && response.Event != types.EventStoreSetResponse {
		return
	}
	c.router.Deliver(response.CorrelationID, response)
}
