package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A JobContext built by hand, as existing jobs' tests do, must behave like a
// context that is never cancelled.
func TestJobContextWithoutContextIsNeverCancelled(t *testing.T) {
	ctx := &JobContext{RunID: "r", Args: map[string]interface{}{"a": "b"}}
	if ctx.IsCancelled() || ctx.Err() != nil {
		t.Fatalf("zero JobContext reports cancelled: %v", ctx.Err())
	}
	select {
	case <-ctx.Done():
		t.Fatal("Done closed on a JobContext without a context")
	default:
	}
	if ctx.Context() == nil {
		t.Fatal("Context() returned nil")
	}
	if got := ctx.GetStringArg("a", ""); got != "b" {
		t.Fatalf("GetStringArg = %q", got)
	}
}

func TestJobContextCarriesStopSignal(t *testing.T) {
	parent, cancel := context.WithCancelCause(context.Background())
	base := &JobContext{RunID: "r"}
	ctx := base.WithContext(parent)
	if base.ctx != nil {
		t.Fatal("WithContext mutated the receiver")
	}
	if ctx.RunID != "r" {
		t.Fatalf("WithContext lost fields: %+v", ctx)
	}

	// Usable wherever a context.Context is taken.
	var asCtx context.Context = ctx
	cancel(ErrRunCancelled)

	select {
	case <-asCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("Done not closed after cancel")
	}
	if !ctx.IsCancelled() || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("after cancel: IsCancelled=%v Err=%v", ctx.IsCancelled(), ctx.Err())
	}
	if !errors.Is(context.Cause(ctx.Context()), ErrRunCancelled) {
		t.Fatalf("cause = %v, want ErrRunCancelled", context.Cause(ctx.Context()))
	}
}
