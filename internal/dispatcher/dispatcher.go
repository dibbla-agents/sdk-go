package dispatcher

import (
	"github.com/dibbla-agents/sdk-go/internal/types"
	"log"
	"sync"
)

// Handler processes a single event message.
type Handler func(*types.EventMessage)

// Dispatcher routes events to registered handlers and executes them via a worker pool.
//
// Handlers registered with RegisterDirect bypass the pool entirely and run
// inline on the dispatching goroutine. This exists because pooled handlers may
// make nested synchronous request/response calls back to the server; the
// response events that unpark them must never queue behind requests in the
// same bounded pool, or the pool deadlocks once all workers are waiting
// (FAT-19). Direct handlers must therefore be fast and non-blocking.
type Dispatcher struct {
	mu       sync.RWMutex // guards registries: handlers register after Start()'s workers are live
	registry map[string]Handler
	direct   map[string]Handler
	jobs     chan *types.EventMessage
	wg       sync.WaitGroup
}

// NewDispatcher creates a dispatcher with a bounded queue of the given size.
func NewDispatcher(queueSize int) *Dispatcher {
	return &Dispatcher{
		registry: make(map[string]Handler),
		direct:   make(map[string]Handler),
		jobs:     make(chan *types.EventMessage, queueSize),
	}
}

// Register associates an event name with a handler executed by the worker pool.
func (d *Dispatcher) Register(event string, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.registry[event] = handler
}

// RegisterDirect associates an event name with a handler executed inline by
// Dispatch, bypassing the worker pool and its queue. Use it for handlers that
// are fast and never block: correlation responses, liveness/control replies.
func (d *Dispatcher) RegisterDirect(event string, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.direct[event] = handler
}

// lookup returns the handler for an event and whether it is direct.
func (d *Dispatcher) lookup(event string) (handler Handler, direct bool, ok bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if handler, ok := d.direct[event]; ok {
		return handler, true, true
	}
	handler, ok = d.registry[event]
	return handler, false, ok
}

// Start launches n worker goroutines to process queued events.
func (d *Dispatcher) Start(n int) {
	if n <= 0 {
		n = 1
	}
	d.wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer d.wg.Done()
			for msg := range d.jobs {
				if handler, _, ok := d.lookup(msg.Event); ok {
					handler(msg)
				} else {
					log.Printf("No handler registered for event: %s", msg.Event)
				}
			}
		}()
	}
}

// Stop stops accepting new jobs and waits for workers to finish.
func (d *Dispatcher) Stop() {
	close(d.jobs)
	d.wg.Wait()
}

// Dispatch routes a message: direct handlers run inline, everything else is
// enqueued for the worker pool without blocking. It reports whether the
// message was handled or enqueued; false means the queue was full and the
// message was not accepted — the caller decides how to surface the overflow.
// Never blocking here keeps the single inbound consumer loop live even when
// the pool is saturated (FAT-19).
func (d *Dispatcher) Dispatch(msg *types.EventMessage) bool {
	if handler, direct, ok := d.lookup(msg.Event); ok && direct {
		handler(msg)
		return true
	}
	select {
	case d.jobs <- msg:
		return true
	default:
		return false
	}
}
