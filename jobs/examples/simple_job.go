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
	}
}

// Execute runs the job with the given context.
// The context provides access to arguments, logger, and run information.
func (j *SimpleJob) Execute(ctx *jobs.JobContext) error {
	message := ctx.GetStringArg("message", "Hello from SimpleJob")

	ctx.Logger.Info(fmt.Sprintf("Starting simple job with message: %s", message))

	// Task 1: Fetch Data
	ctx.Logger.TaskStarted("fetch_data")
	ctx.Logger.Info("Fetching data from external source...")
	time.Sleep(1 * time.Second) // Simulate API call
	ctx.Logger.Info("Successfully fetched 100 records")
	ctx.Logger.TaskCompleted()

	// Task 2: Process Data with Progress
	ctx.Logger.TaskStarted("process_data")
	ctx.Logger.Info("Processing fetched data...")

	total := 20
	for i := 1; i <= total; i++ {
		time.Sleep(100 * time.Millisecond) // Simulate work
		ctx.Logger.Progress(i, total, fmt.Sprintf("Processing record %d/%d", i, total))
	}
	ctx.Logger.CompleteProgress()
	ctx.Logger.Info("Data processing completed")
	ctx.Logger.TaskCompleted()

	// Task 3: Save Results
	ctx.Logger.TaskStarted("save_results")
	ctx.Logger.Info("Saving results...")
	time.Sleep(500 * time.Millisecond) // Simulate database write
	ctx.Logger.Info("Successfully saved 100 records")
	ctx.Logger.TaskCompleted()

	ctx.Logger.Info("Simple job completed successfully!")
	return nil
}

// ExampleSimpleJobUsage shows how to register and use SimpleJob with a JobHost.
//
// Usage with SDK server:
//
//	server, _ := sdk.New(
//	    sdk.WithServerName("my-worker"),
//	    sdk.WithServerApiToken(os.Getenv("SERVER_API_TOKEN")),
//	)
//
//	jobHost := jobs.NewJobHost(server.GetGlobalState(), "my-job-host")
//	jobHost.RegisterJob(&examples.SimpleJob{})
//	jobHost.Start()
//
//	server.Start()
func ExampleSimpleJobUsage() {
	// This is a documentation example - see the function comment for usage
}
