package dispatcher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dibbla-agents/sdk-go/internal/types"
)

// TestRegisterAfterStartIsRaceFree reproduces the SDK's real startup order:
// Start() launches workers first, handlers register afterwards while events
// already flow. Pre-fix this was an unsynchronised map read/write — run with
// -race to catch regressions.
func TestRegisterAfterStartIsRaceFree(t *testing.T) {
	d := NewDispatcher(128)
	d.Start(8)

	var handled atomic.Int64
	var wg sync.WaitGroup

	// Concurrently register handlers...
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			d.Register("event-a", func(*types.EventMessage) { handled.Add(1) })
			d.Register("event-b", func(*types.EventMessage) { handled.Add(1) })
		}
	}()

	// ...while events are dispatched.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			d.Dispatch(&types.EventMessage{Event: "event-a"})
		}
	}()

	wg.Wait()
	d.Stop()

	if handled.Load() == 0 {
		t.Fatal("no events were handled")
	}
}

// TestDirectHandlerBypassesSaturatedPool reproduces the FAT-19 deadlock: every
// pool worker is parked waiting for a response event, and the queue is full of
// requests. Pre-fix the response had to pass through the same pool and could
// never run. With RegisterDirect the response runs inline on the dispatching
// goroutine, unparks the workers, and the system drains.
func TestDirectHandlerBypassesSaturatedPool(t *testing.T) {
	const workers = 4
	d := NewDispatcher(workers) // small queue so saturation is easy to reach
	release := make(chan struct{})

	// Pooled handler: blocks like a nested synchronous rpc/store call.
	d.Register("request", func(*types.EventMessage) { <-release })
	// Direct handler: the "response" that unparks the workers.
	d.RegisterDirect("response", func(*types.EventMessage) { close(release) })

	d.Start(workers)

	// Park every worker and fill the queue.
	for i := 0; i < workers*2; i++ {
		if !d.Dispatch(&types.EventMessage{Event: "request"}) {
			break // queue full — saturated, as intended
		}
	}

	// The response must still get through, without blocking the caller.
	done := make(chan struct{})
	go func() {
		d.Dispatch(&types.EventMessage{Event: "response"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("direct dispatch blocked behind a saturated worker pool (FAT-19 deadlock)")
	}
	d.Stop() // drains: workers were released by the direct handler
}

// TestDispatchDoesNotBlockWhenQueueFull verifies the consumer loop can never
// wedge on a full queue: Dispatch must return false instead of blocking.
func TestDispatchDoesNotBlockWhenQueueFull(t *testing.T) {
	d := NewDispatcher(1)
	release := make(chan struct{})
	d.Register("busy", func(*types.EventMessage) { <-release })
	d.Start(1)

	// First occupies the worker, second fills the queue (allow for scheduling:
	// keep dispatching until one is rejected).
	deadline := time.After(2 * time.Second)
	for d.Dispatch(&types.EventMessage{Event: "busy"}) {
		select {
		case <-deadline:
			t.Fatal("Dispatch never reported a full queue")
		default:
		}
	}

	close(release)
	d.Stop()
}

// TestDirectHandlerRunsInline verifies a direct handler executes on the
// dispatching goroutine and reports success even when the queue is full.
func TestDirectHandlerRunsInline(t *testing.T) {
	d := NewDispatcher(1) // no workers started: pool is completely inert
	var ran atomic.Bool
	d.RegisterDirect("ping", func(*types.EventMessage) { ran.Store(true) })

	if !d.Dispatch(&types.EventMessage{Event: "ping"}) {
		t.Fatal("direct dispatch reported failure")
	}
	if !ran.Load() {
		t.Fatal("direct handler did not run inline")
	}
}
