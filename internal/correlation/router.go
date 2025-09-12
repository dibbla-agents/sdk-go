package correlation

import (
	"context"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/maps"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
)

// Router manages correlated request/response channels by correlation ID.
type Router struct {
	channels *maps.SafeFunctionMap[string, chan types.EventMessage]
}

// NewRouter creates a new correlation router.
func NewRouter() *Router {
	return &Router{channels: maps.NewSafeFunctionMap[string, chan types.EventMessage]()}
}

// Register allocates and stores a response channel for the given ID.
func (r *Router) Register(id string, buffer int) chan types.EventMessage {
	ch := make(chan types.EventMessage, buffer)
	r.channels.Store(id, ch)
	return ch
}

// Remove deletes the channel registration for the given ID.
func (r *Router) Remove(id string) {
	r.channels.Delete(id)
}

// Deliver sends a message to the registered channel for the given ID, if present.
func (r *Router) Deliver(id string, msg types.EventMessage) {
	if ch, ok := r.channels.Load(id); ok {
		ch <- msg
	}
}

// Await waits for a response on the provided channel or the context to end.
func (r *Router) Await(ctx context.Context, ch chan types.EventMessage) (types.EventMessage, bool) {
	select {
	case resp := <-ch:
		return resp, true
	case <-ctx.Done():
		var zero types.EventMessage
		return zero, false
	}
}
