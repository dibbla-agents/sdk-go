package dispatcher

import (
	"sync"
	"sync/atomic"
	"testing"

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
