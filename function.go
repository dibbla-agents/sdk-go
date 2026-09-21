package sdk

import (
	"context"
	"fmt"
	"time"

	"github.com/dibbla-agents/sdk-go/internal/basefunction"
	"github.com/dibbla-agents/sdk-go/internal/state"
	"github.com/dibbla-agents/sdk-go/internal/types"
)

// FunctionBuilder defines the interface for building functions
type FunctionBuilder interface {
	Build(gs *state.GlobalState) basefunction.FunctionInterface
}

// Function provides a fluent interface for building functions with type safety
type Function[In any, Out any] struct {
	name        string
	version     string
	description string
	handler     func(In, *types.EventMessage, *state.GlobalState) (Out, error)
	tags        []string
	ttl         time.Duration
}

// NewFunction creates a new function builder with the specified name, version, and description
func NewFunction[In any, Out any](name, version, description string) *Function[In, Out] {
	return &Function[In, Out]{
		name:        name,
		version:     version,
		description: description,
		tags:        []string{},
		ttl:         0,
	}
}

// WithHandler sets the function handler
// The handler receives the typed input, event message, and global state
func (f *Function[In, Out]) WithHandler(handler func(In, *types.EventMessage, *state.GlobalState) (Out, error)) *Function[In, Out] {
	f.handler = handler
	return f
}

// WithTags adds tags to the function for categorization and filtering
func (f *Function[In, Out]) WithTags(tags ...string) *Function[In, Out] {
	f.tags = append(f.tags, tags...)
	return f
}

// WithCacheTTL sets the per-function cache TTL. A value of 0 disables caching.
func (f *Function[In, Out]) WithCacheTTL(ttl time.Duration) *Function[In, Out] {
	f.ttl = ttl
	return f
}

// Build creates the actual function implementation that satisfies basefunction.FunctionInterface
func (f *Function[In, Out]) Build(gs *state.GlobalState) basefunction.FunctionInterface {
	bf := basefunction.NewFunction(
		f.name,
		f.version,
		f.description,
		func(inputs In, eventState *types.EventMessage) (Out, error) {
			// Call the user's handler with the global state for advanced use cases
			return f.handler(inputs, eventState, gs)
		},
		f.tags,
	)

	// Apply TTL if set (including 0 to explicitly disable caching)
	switch fn := any(bf).(type) {
	case interface{ SetCacheTTL(time.Duration) }:
		fn.SetCacheTTL(f.ttl)
	}

	return bf
}

// SimpleFunction provides an even simpler interface for functions that don't need event state or global state
type SimpleFunction[In any, Out any] struct {
	name           string
	version        string
	description    string
	handler        func(In) (Out, error)
	contextHandler func(context.Context, In) (Out, error)
	tags           []string
}

// NewSimpleFunction creates a function builder for simple input->output transformations
func NewSimpleFunction[In any, Out any](name, version, description string) *SimpleFunction[In, Out] {
	return &SimpleFunction[In, Out]{
		name:        name,
		version:     version,
		description: description,
		tags:        []string{},
	}
}

// WithHandler sets the simple function handler.
func (f *SimpleFunction[In, Out]) WithHandler(handler func(In) (Out, error)) *SimpleFunction[In, Out] {
	f.handler = handler
	f.contextHandler = nil
	return f
}

// WithContextHandler sets a handler that also receives a context carrying the
// verified caller, readable with CallerFromContext. Use it whenever the
// function authorizes per user, or needs to hand a context to the I/O it
// performs.
//
// It replaces any handler set by WithHandler — a function has one handler, and
// silently preferring one of two would make which code runs depend on call
// order.
//
// The context is not cancellable today: the invocation path carries no
// deadline, so it is a value carrier for now. Accepting it anyway is what lets
// a deadline arrive later without changing this signature.
func (f *SimpleFunction[In, Out]) WithContextHandler(handler func(context.Context, In) (Out, error)) *SimpleFunction[In, Out] {
	f.contextHandler = handler
	f.handler = nil
	return f
}

// WithTags adds tags to the function
func (f *SimpleFunction[In, Out]) WithTags(tags ...string) *SimpleFunction[In, Out] {
	f.tags = append(f.tags, tags...)
	return f
}

// Build creates the actual function implementation
func (f *SimpleFunction[In, Out]) Build(gs *state.GlobalState) basefunction.FunctionInterface {
	return basefunction.NewFunction(
		f.name,
		f.version,
		f.description,
		func(inputs In, eventState *types.EventMessage) (Out, error) {
			switch {
			case f.contextHandler != nil:
				// The caller travels in the event's meta, outside the payload,
				// so a caller cannot forge it by shaping its inputs.
				return f.contextHandler(contextForEvent(eventState), inputs)
			case f.handler != nil:
				// Simple handler ignores event state and global state
				return f.handler(inputs)
			default:
				// Registering a function with no handler is a wiring mistake.
				// Answering with it beats a nil dereference that surfaces as a
				// worker panic with no function name attached.
				var zero Out
				return zero, fmt.Errorf("function %q (version %s) was registered without a handler", f.name, f.version)
			}
		},
		f.tags,
	)
}
