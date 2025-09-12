package grpccache

import (
	"context"
	"fmt"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/communication"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/correlation"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/utils"
	"strconv"
	"time"
)

// Client provides a cache client over the workflow gRPC stream using correlation IDs
type Client struct {
	communicator   communication.WorkflowCommunicator
	router         *correlation.Router
	defaultTimeout time.Duration
	serverName     string
}

// NewClient creates a new gRPC-based cache client
func NewClient(comm communication.WorkflowCommunicator, serverName string) *Client {
	return &Client{
		communicator:   comm,
		router:         correlation.NewRouter(),
		defaultTimeout: 30 * time.Second,
		serverName:     serverName,
	}
}

// GetByString requests a cache value by string key and waits for a cache_get_response with the same correlation ID
// The response value is returned in the payload
func (c *Client) GetByString(ctx context.Context, key string) ([]byte, error) {
	if c == nil || c.communicator == nil {
		return nil, fmt.Errorf("grpccache: no communicator configured")
	}

	correlationID := utils.UID()
	responseChan := c.router.Register(correlationID, 1)
	defer c.router.Remove(correlationID)

	meta := map[string]any{
		"Key":            key,
		"calling_server": c.serverName,
	}

	event := types.EventMessage{
		Event:         types.EventCacheGetRequest,
		Text:          "Cache get request",
		Meta:          &meta,
		Payload:       nil,
		CorrelationID: correlationID,
	}

	if err := c.communicator.SendEvent(&event); err != nil {
		return nil, fmt.Errorf("grpccache: failed to send cache_get_request: %w", err)
	}

	select {
	case resp := <-responseChan:
		if resp.Payload == nil {
			return nil, fmt.Errorf("grpccache: empty cache_get_response payload")
		}
		return *resp.Payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetUint64 requests a cache value by numeric key
func (c *Client) GetUint64(ctx context.Context, key uint64) ([]byte, error) {
	return c.GetByString(ctx, strconv.FormatUint(key, 10))
}

// Get provides a redis.FunctionCache-compatible method signature.
// It returns (value, true) if found. If the server returns an empty or missing payload, it returns (nil, false).
func (c *Client) GetCompat(ctx context.Context, key uint64) ([]byte, bool) {
	data, err := c.GetByString(ctx, strconv.FormatUint(key, 10))
	if err != nil {
		return nil, false
	}
	if len(data) == 0 {
		return nil, false
	}
	return data, true
}

// Get implements basefunction.FunctionCache: uses an internal timeout and returns (value, true) if received
func (c *Client) Get(key uint64) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), c.defaultTimeout)
	defer cancel()
	data, err := c.GetByString(ctx, strconv.FormatUint(key, 10))
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}

// SetByString stores a cache value with a TTL (in seconds) as part of the Meta and the value in the payload
// This is sent as a fire-and-forget "cache_set" event; no response is required
func (c *Client) SetByString(ctx context.Context, key string, value []byte, ttlSeconds int64) error {
	if c == nil || c.communicator == nil {
		return fmt.Errorf("grpccache: no communicator configured")
	}

	// Copy value into a new slice to avoid accidental mutation after send
	payloadCopy := make([]byte, len(value))
	copy(payloadCopy, value)

	meta := map[string]any{
		"Key":            key,
		"TTL":            ttlSeconds,
		"calling_server": c.serverName,
	}

	event := types.EventMessage{
		Event:         types.EventCacheSet,
		Text:          "Cache set",
		Meta:          &meta,
		Payload:       &payloadCopy,
		CorrelationID: utils.UID(),
	}

	if err := c.communicator.SendEvent(&event); err != nil {
		return fmt.Errorf("grpccache: failed to send cache_set: %w", err)
	}
	return nil
}

// SetUint64 stores a cache value by numeric key with a time.Duration TTL
func (c *Client) SetUint64(ctx context.Context, key uint64, value []byte, ttl time.Duration) error {
	return c.SetByString(ctx, strconv.FormatUint(key, 10), value, int64(ttl.Seconds()))
}

// Set provides a redis.FunctionCache-compatible method signature using the default TTL semantics on the server
func (c *Client) SetCompat(ctx context.Context, key uint64, value []byte) error {
	return c.SetUint64(ctx, key, value, 0)
}

// SetWithTTL provides a redis.FunctionCache-compatible method signature
func (c *Client) SetWithTTLCompat(ctx context.Context, key uint64, value []byte, ttl time.Duration) error {
	return c.SetUint64(ctx, key, value, ttl)
}

// Set implements basefunction.FunctionCache: fire-and-forget with default TTL behavior
func (c *Client) Set(key uint64, value []byte) error {
	return c.SetUint64(context.Background(), key, value, 0)
}

// SetWithTTL implements basefunction.FunctionCache
func (c *Client) SetWithTTL(key uint64, value []byte, ttl time.Duration) error {
	return c.SetUint64(context.Background(), key, value, ttl)
}

// HandleResponse delivers cache response events to the waiting goroutine using the correlation ID
// Supported events: cache_get_response (and optionally cache_set_response if server sends it)
func (c *Client) HandleResponse(response types.EventMessage) {
	if c == nil {
		return
	}
	if response.Event != types.EventCacheGetResponse && response.Event != types.EventCacheSetResponse {
		return
	}
	c.router.Deliver(response.CorrelationID, response)
}
