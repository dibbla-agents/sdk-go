package jobs

import (
	"context"
	"errors"
	"time"
)

// ErrRunCancelled is the cancellation cause when someone stops the run from
// the console or the API (DIB-1381). ctx.Err() is context.Canceled as usual;
// context.Cause(ctx.Context()) is this error.
var ErrRunCancelled = errors.New("run cancelled by user")

// JobContext provides context for job execution.
//
// It is also a context.Context: it is cancelled when the run is stopped from
// the console or the API. Pass it to anything that takes a context
// (http.NewRequestWithContext, database/sql, gRPC calls) and check
// ctx.Done() between units of work, so the job stops within seconds instead
// of writing data nobody wants any more. A job that never looks at it runs to
// completion exactly as before.
type JobContext struct {
	RunID   string
	JobID   string
	JobName string
	Args    map[string]interface{}
	Logger  *Logger

	// ctx carries the stop signal. Nil (e.g. a JobContext built by hand in a
	// test) behaves like context.Background(): never cancelled.
	ctx context.Context
}

// WithContext returns a shallow copy of the JobContext whose stop signal is
// ctx. The SDK uses it to wire the run's cancellation in; tests can use it to
// simulate a stop.
func (ctx *JobContext) WithContext(c context.Context) *JobContext {
	cp := *ctx
	cp.ctx = c
	return &cp
}

// Context returns the run's context.Context, cancelled when the run is
// stopped. Never nil.
func (ctx *JobContext) Context() context.Context {
	if ctx == nil || ctx.ctx == nil {
		return context.Background()
	}
	return ctx.ctx
}

// Done is closed when the run is stopped. Same contract as context.Context.
func (ctx *JobContext) Done() <-chan struct{} { return ctx.Context().Done() }

// Err is nil while the run may continue and context.Canceled once it has been
// stopped. Same contract as context.Context.
func (ctx *JobContext) Err() error { return ctx.Context().Err() }

// Deadline implements context.Context. Runs have no deadline today.
func (ctx *JobContext) Deadline() (time.Time, bool) { return ctx.Context().Deadline() }

// Value implements context.Context.
func (ctx *JobContext) Value(key any) any { return ctx.Context().Value(key) }

// IsCancelled reports whether the run has been stopped. Convenient in loops:
//
//	for _, row := range rows {
//	    if ctx.IsCancelled() {
//	        return ctx.Err()
//	    }
//	    write(row)
//	}
func (ctx *JobContext) IsCancelled() bool { return ctx.Err() != nil }

var _ context.Context = (*JobContext)(nil)

// GetArg retrieves an argument by name with type assertion
func (ctx *JobContext) GetArg(name string) (interface{}, bool) {
	if ctx.Args == nil {
		return nil, false
	}
	val, ok := ctx.Args[name]
	return val, ok
}

// GetStringArg retrieves a string argument
func (ctx *JobContext) GetStringArg(name string, defaultVal string) string {
	if val, ok := ctx.GetArg(name); ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultVal
}

// GetIntArg retrieves an integer argument
func (ctx *JobContext) GetIntArg(name string, defaultVal int) int {
	if val, ok := ctx.GetArg(name); ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return defaultVal
}

// GetBoolArg retrieves a boolean argument
func (ctx *JobContext) GetBoolArg(name string, defaultVal bool) bool {
	if val, ok := ctx.GetArg(name); ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// GetFloat64Arg retrieves a float64 argument
func (ctx *JobContext) GetFloat64Arg(name string, defaultVal float64) float64 {
	if val, ok := ctx.GetArg(name); ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		}
	}
	return defaultVal
}
