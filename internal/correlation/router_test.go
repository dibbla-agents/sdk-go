package correlation

import (
	"testing"
	"time"

	"github.com/dibbla-agents/sdk-go/internal/types"
)

// TestDeliverDuplicateDoesNotBlock is the FAT-19 escalation-path regression:
// a duplicate response to a buffer-1 channel whose waiter already consumed
// (or never will consume) must not park the delivering goroutine forever.
func TestDeliverDuplicateDoesNotBlock(t *testing.T) {
	r := NewRouter()
	ch := r.Register("dup-id", 1)

	done := make(chan struct{})
	go func() {
		r.Deliver("dup-id", types.EventMessage{Text: "first"})  // fills the buffer
		r.Deliver("dup-id", types.EventMessage{Text: "second"}) // pre-fix: blocks forever
		r.Deliver("dup-id", types.EventMessage{Text: "third"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Deliver blocked on a full channel")
	}

	// The first message must still have landed.
	select {
	case msg := <-ch:
		if msg.Text != "first" {
			t.Fatalf("got %q, want %q", msg.Text, "first")
		}
	default:
		t.Fatal("first message was not delivered")
	}
}

// TestDeliverUnknownIDIsNoop verifies delivery to an unregistered/removed ID
// neither blocks nor panics.
func TestDeliverUnknownIDIsNoop(t *testing.T) {
	r := NewRouter()
	ch := r.Register("gone", 1)
	r.Remove("gone")

	done := make(chan struct{})
	go func() {
		r.Deliver("gone", types.EventMessage{Text: "late"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Deliver blocked for removed ID")
	}
	select {
	case <-ch:
		t.Fatal("message delivered to removed channel")
	default:
	}
}
