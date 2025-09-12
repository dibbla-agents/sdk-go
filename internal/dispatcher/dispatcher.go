package dispatcher

import (
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
	"log"
	"sync"
)

// Handler processes a single event message.
type Handler func(*types.EventMessage)

// Dispatcher routes events to registered handlers and executes them via a worker pool.
type Dispatcher struct {
	registry map[string]Handler
	jobs     chan *types.EventMessage
	wg       sync.WaitGroup
}

// NewDispatcher creates a dispatcher with a bounded queue of the given size.
func NewDispatcher(queueSize int) *Dispatcher {
	return &Dispatcher{
		registry: make(map[string]Handler),
		jobs:     make(chan *types.EventMessage, queueSize),
	}
}

// Register associates an event name with a handler.
func (d *Dispatcher) Register(event string, handler Handler) { d.registry[event] = handler }

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
				if handler, ok := d.registry[msg.Event]; ok {
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

// Dispatch enqueues a message for processing.
func (d *Dispatcher) Dispatch(msg *types.EventMessage) { d.jobs <- msg }
