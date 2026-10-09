// Package examples provides example job implementations for the jobs subpackage.
//
// This file contains a simple job example demonstrating basic job execution
// with tasks and progress reporting via gRPC.
package examples

import (
	"fmt"
	"time"

	"github.com/dibbla-agents/sdk-go/jobs"
)

// SimpleJob demonstrates basic job execution with multiple tasks.
// It implements the jobs.JobHandler interface.
type SimpleJob struct{}

// GetJobID returns the unique identifier for this job type.
func (j *SimpleJob) GetJobID() string {
	return "simple_job"
}

// GetJobName returns the human-readable name for this job.
func (j *SimpleJob) GetJobName() string {
	return "Simple Data Processing Job"
}

// GetParameters returns the parameters this job accepts.
func (j *SimpleJob) GetParameters() []jobs.JobParameter {
	return []jobs.JobParameter{
		{Name: "message", Type: "string", Required: false, Default: "Hello from SimpleJob"},
		{Name: "records", Type: "integer", Required: false, Default: 20},
	}
}

// Execute runs the job with the given context.
// The context provides access to arguments, logger, and run information.
//
// It also shows how a job honours Stop: ctx is a context.Context that is
// cancelled when someone stops the run in the console. Waits go through
// sleep (below), which returns early on a stop, and the loop checks between
// records, so the job stops within one record instead of finishing the run.
// Returning ctx.Err() ends the run as "cancelled".
func (j *SimpleJob) Execute(ctx *jobs.JobContext) error {
	message := ctx.GetStringArg("message", "Hello from SimpleJob")
	total := ctx.GetIntArg("records", 20)

	ctx.Logger.Info(fmt.Sprintf("Starting simple job with message: %s", message))

	// Task 1: Fetch Data
	ctx.Logger.TaskStarted("fetch_data")
	ctx.Logger.Info("Fetching data from external source...")
	if err := sleep(ctx, 1*time.Second); err != nil { // Simulate API call
		return err
	}
	ctx.Logger.Info(fmt.Sprintf("Successfully fetched %d records", total))
	ctx.Logger.TaskCompleted()

	// Task 2: Process Data with Progress
	ctx.Logger.TaskStarted("process_data")
	ctx.Logger.Info("Processing fetched data...")

	for i := 1; i <= total; i++ {
		// Check between units of work: stop before writing the next record.
		if ctx.IsCancelled() {
			ctx.Logger.Warn(fmt.Sprintf("Stopped after %d of %d records", i-1, total))
			return ctx.Err()
		}
		if err := sleep(ctx, 100*time.Millisecond); err != nil { // Simulate work
			ctx.Logger.Warn(fmt.Sprintf("Stopped after %d of %d records", i-1, total))
			return err
		}
		ctx.Logger.Progress(i, total, fmt.Sprintf("Processing record %d/%d", i, total))
	}
	ctx.Logger.CompleteProgress()
	ctx.Logger.Info("Data processing completed")
	ctx.Logger.TaskCompleted()

	// Task 3: Save Results
	ctx.Logger.TaskStarted("save_results")
	ctx.Logger.Info("Saving results...")
	if err := sleep(ctx, 500*time.Millisecond); err != nil { // Simulate database write
		return err
	}
	ctx.Logger.Info(fmt.Sprintf("Successfully saved %d records", total))
	ctx.Logger.TaskCompleted()

	ctx.Logger.Info("Simple job completed successfully!")
	return nil
}

// sleep waits for d, or returns ctx.Err() as soon as the run is stopped.
// Use the same select around any wait in your own jobs.
func sleep(ctx *jobs.JobContext, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// ExampleSimpleJobUsage shows how to register and use SimpleJob.
//
// Usage with SDK server:
//
//	server, _ := sdk.New(
//	    sdk.WithServerName("my-worker"),
//	    sdk.WithServerApiToken(os.Getenv("SERVER_API_TOKEN")),
//	)
//
//	// Register job directly with the server
//	server.RegisterJob(&examples.SimpleJob{})
//
//	// Start handles everything: connection, registration, and blocking
//	server.Start()
func ExampleSimpleJobUsage() {
	// This is a documentation example - see the function comment for usage
}
