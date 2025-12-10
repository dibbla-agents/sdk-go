# Jobs Examples

This directory contains example job implementations demonstrating how to use the `jobs` subpackage.

## Examples

### SimpleJob (`simple_job.go`)

A basic job demonstrating:
- Implementing the `JobHandler` interface
- Multiple task execution with `TaskStarted`/`TaskCompleted`
- Progress reporting with `Progress()`
- Basic logging with `Info()`

**Parameters:**
- `message` (string, optional): Custom message to display

### DataProcessingJob (`data_processing_job.go`)

A complex data processing pipeline demonstrating:
- Multiple job parameters with different types
- Multi-phase processing (fetch → validate → transform → load → report)
- Conditional task execution (skipping validation)
- Different log levels (`Info`, `Warn`, `Error`)
- Dry-run mode support
- Comprehensive progress reporting across phases

**Parameters:**
- `source` (string, required): Data source location
- `destination` (string, required): Output destination
- `batch_size` (integer, optional, default: 100): Processing batch size
- `validate` (boolean, optional, default: true): Enable validation
- `dry_run` (boolean, optional, default: false): Run without making changes

## Usage

### Basic Setup

```go
package main

import (
    "os"

    "github.com/dibbla-agents/sdk-go"
    "github.com/dibbla-agents/sdk-go/jobs/examples"
)

func main() {
    // Create SDK server
    server, err := sdk.New(
        sdk.WithServerName("my-worker"),
        sdk.WithServerApiToken(os.Getenv("SERVER_API_TOKEN")),
    )
    if err != nil {
        panic(err)
    }

    // Register jobs directly with the server
    server.RegisterJob(&examples.SimpleJob{})
    server.RegisterJob(&examples.DataProcessingJob{})

    // Start server - handles connection, registration, and blocking
    server.Start()
}
```

### Implementing Your Own Job

```go
type MyJob struct{}

func (j *MyJob) GetJobID() string   { return "my_job" }
func (j *MyJob) GetJobName() string { return "My Custom Job" }

func (j *MyJob) GetParameters() []jobs.JobParameter {
    return []jobs.JobParameter{
        {Name: "input", Type: "string", Required: true},
        {Name: "count", Type: "integer", Required: false, Default: 10},
    }
}

func (j *MyJob) Execute(ctx *jobs.JobContext) error {
    input := ctx.GetStringArg("input", "")
    count := ctx.GetIntArg("count", 10)

    ctx.Logger.Info(fmt.Sprintf("Processing %s with count %d", input, count))

    ctx.Logger.TaskStarted("process")
    for i := 1; i <= count; i++ {
        ctx.Logger.Progress(i, count, "Processing...")
        // Do work here
    }
    ctx.Logger.CompleteProgress()
    ctx.Logger.TaskCompleted()

    return nil
}
```

## Logger Methods

| Method | Description |
|--------|-------------|
| `Info(msg)` | Log informational message |
| `Warn(msg)` | Log warning message |
| `Error(msg)` | Log error message |
| `TaskStarted(name)` | Mark task as started |
| `TaskCompleted()` | Mark current task as completed |
| `TaskFailed(err)` | Mark current task as failed |
| `TaskSkipped(reason)` | Mark current task as skipped |
| `Progress(current, total, msg)` | Report progress with known total |
| `ProgressIndeterminate(current, msg)` | Report progress without total |
| `CompleteProgress()` | Complete progress bar |

## JobContext Argument Helpers

| Method | Description |
|--------|-------------|
| `GetArg(name)` | Get raw argument value |
| `GetStringArg(name, default)` | Get string argument with default |
| `GetIntArg(name, default)` | Get integer argument with default |
| `GetBoolArg(name, default)` | Get boolean argument with default |
| `GetFloat64Arg(name, default)` | Get float64 argument with default |
