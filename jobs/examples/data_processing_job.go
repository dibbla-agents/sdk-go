// Package examples provides example job implementations for the jobs subpackage.
//
// This file contains a complex job example demonstrating advanced features
// like multiple phases, error handling, different log levels, and
// comprehensive progress reporting.
package examples

import (
	"fmt"
	"time"

	"github.com/dibbla-agents/sdk-go/jobs"
)

// DataProcessingJob demonstrates a complex job with multiple phases,
// progress reporting, and error handling. It implements jobs.JobHandler.
type DataProcessingJob struct{}

// GetJobID returns the unique identifier for this job type.
func (j *DataProcessingJob) GetJobID() string {
	return "data_processing"
}

// GetJobName returns the human-readable name for this job.
func (j *DataProcessingJob) GetJobName() string {
	return "Data Processing Pipeline"
}

// GetParameters returns the parameters this job accepts.
func (j *DataProcessingJob) GetParameters() []jobs.JobParameter {
	return []jobs.JobParameter{
		{Name: "source", Type: "string", Required: true},
		{Name: "destination", Type: "string", Required: true},
		{Name: "batch_size", Type: "integer", Required: false, Default: 100},
		{Name: "validate", Type: "boolean", Required: false, Default: true},
		{Name: "dry_run", Type: "boolean", Required: false, Default: false},
	}
}

// Execute runs the data processing pipeline.
func (j *DataProcessingJob) Execute(ctx *jobs.JobContext) error {
	// Extract arguments with defaults
	source := ctx.GetStringArg("source", "")
	destination := ctx.GetStringArg("destination", "")
	batchSize := ctx.GetIntArg("batch_size", 100)
	validate := ctx.GetBoolArg("validate", true)
	dryRun := ctx.GetBoolArg("dry_run", false)

	if source == "" || destination == "" {
		return fmt.Errorf("source and destination are required")
	}

	ctx.Logger.Info(fmt.Sprintf("Starting data processing pipeline"))
	ctx.Logger.Info(fmt.Sprintf("  Source: %s", source))
	ctx.Logger.Info(fmt.Sprintf("  Destination: %s", destination))
	ctx.Logger.Info(fmt.Sprintf("  Batch size: %d", batchSize))
	ctx.Logger.Info(fmt.Sprintf("  Validation: %v", validate))
	ctx.Logger.Info(fmt.Sprintf("  Dry run: %v", dryRun))

	if dryRun {
		ctx.Logger.Warn("DRY RUN MODE - No actual changes will be made")
	}

	// Phase 1: Download/Fetch Data
	if err := j.fetchData(ctx, source); err != nil {
		return fmt.Errorf("fetch phase failed: %w", err)
	}

	// Phase 2: Validate Data (optional)
	if validate {
		if err := j.validateData(ctx); err != nil {
			return fmt.Errorf("validation phase failed: %w", err)
		}
	} else {
		ctx.Logger.TaskStarted("validate_data")
		ctx.Logger.TaskSkipped("Validation disabled by configuration")
	}

	// Phase 3: Transform Data
	if err := j.transformData(ctx, batchSize); err != nil {
		return fmt.Errorf("transform phase failed: %w", err)
	}

	// Phase 4: Load/Save Data
	if err := j.loadData(ctx, destination, dryRun); err != nil {
		return fmt.Errorf("load phase failed: %w", err)
	}

	// Phase 5: Generate Report
	if err := j.generateReport(ctx); err != nil {
		return fmt.Errorf("report phase failed: %w", err)
	}

	ctx.Logger.Info("Data processing pipeline completed successfully!")
	return nil
}

// fetchData simulates downloading data from a source
func (j *DataProcessingJob) fetchData(ctx *jobs.JobContext, source string) error {
	ctx.Logger.TaskStarted("fetch_data")
	ctx.Logger.Info(fmt.Sprintf("Connecting to source: %s", source))

	// Simulate downloading multiple files
	files := 5
	for i := 1; i <= files; i++ {
		time.Sleep(300 * time.Millisecond)
		ctx.Logger.Progress(i, files, fmt.Sprintf("Downloading file %d/%d", i, files))
	}
	ctx.Logger.CompleteProgress()

	ctx.Logger.Info(fmt.Sprintf("Downloaded %d files (2.5 GB total)", files))
	ctx.Logger.TaskCompleted()
	return nil
}

// validateData simulates data validation
func (j *DataProcessingJob) validateData(ctx *jobs.JobContext) error {
	ctx.Logger.TaskStarted("validate_data")
	ctx.Logger.Info("Starting data validation...")

	// Simulate validation with some warnings
	records := 1000
	invalidRecords := 0

	for i := 1; i <= 10; i++ {
		time.Sleep(200 * time.Millisecond)
		ctx.Logger.Progress(i, 10, fmt.Sprintf("Validating batch %d/10", i))

		// Simulate finding some invalid records
		if i == 3 || i == 7 {
			invalidRecords += 5
			ctx.Logger.Warn(fmt.Sprintf("Found 5 invalid records in batch %d", i))
		}
	}
	ctx.Logger.CompleteProgress()

	if invalidRecords > 0 {
		ctx.Logger.Warn(fmt.Sprintf("Validation complete: %d/%d records valid (%d invalid)",
			records-invalidRecords, records, invalidRecords))
	} else {
		ctx.Logger.Info(fmt.Sprintf("Validation complete: All %d records valid", records))
	}

	ctx.Logger.TaskCompleted()
	return nil
}

// transformData simulates data transformation
func (j *DataProcessingJob) transformData(ctx *jobs.JobContext, batchSize int) error {
	ctx.Logger.TaskStarted("transform_data")
	ctx.Logger.Info(fmt.Sprintf("Transforming data in batches of %d", batchSize))

	// Simulate multi-phase transformation
	phases := []string{"normalize", "enrich", "aggregate"}

	for _, phase := range phases {
		ctx.Logger.Info(fmt.Sprintf("Running %s transformation...", phase))

		batches := 15
		for i := 1; i <= batches; i++ {
			time.Sleep(80 * time.Millisecond)
			ctx.Logger.Progress(i, batches, fmt.Sprintf("%s: batch %d/%d", phase, i, batches))
		}
		ctx.Logger.CompleteProgress()
	}

	ctx.Logger.Info("Data transformation completed")
	ctx.Logger.TaskCompleted()
	return nil
}

// loadData simulates loading data to destination
func (j *DataProcessingJob) loadData(ctx *jobs.JobContext, destination string, dryRun bool) error {
	ctx.Logger.TaskStarted("load_data")

	if dryRun {
		ctx.Logger.Info(fmt.Sprintf("DRY RUN: Would load data to %s", destination))
		ctx.Logger.Info("DRY RUN: Simulating load without actual writes...")
		time.Sleep(500 * time.Millisecond)
		ctx.Logger.Info("DRY RUN: Load simulation completed")
		ctx.Logger.TaskCompleted()
		return nil
	}

	ctx.Logger.Info(fmt.Sprintf("Loading data to destination: %s", destination))

	// Simulate loading batches
	batches := 20
	for i := 1; i <= batches; i++ {
		time.Sleep(100 * time.Millisecond)
		ctx.Logger.Progress(i, batches, fmt.Sprintf("Loading batch %d/%d", i, batches))
	}
	ctx.Logger.CompleteProgress()

	ctx.Logger.Info("Successfully loaded 1000 records to destination")
	ctx.Logger.TaskCompleted()
	return nil
}

// generateReport simulates report generation
func (j *DataProcessingJob) generateReport(ctx *jobs.JobContext) error {
	ctx.Logger.TaskStarted("generate_report")
	ctx.Logger.Info("Generating processing report...")

	time.Sleep(500 * time.Millisecond)

	// Log summary statistics
	ctx.Logger.Info("=== Processing Summary ===")
	ctx.Logger.Info("  Records processed: 1000")
	ctx.Logger.Info("  Records transformed: 990")
	ctx.Logger.Info("  Records loaded: 990")
	ctx.Logger.Info("  Invalid records: 10")
	ctx.Logger.Info("  Processing time: 12.5s")
	ctx.Logger.Info("========================")

	ctx.Logger.TaskCompleted()
	return nil
}

// ExampleDataProcessingJobUsage shows how to use DataProcessingJob.
//
// Usage with SDK server:
//
//	server, _ := sdk.New(
//	    sdk.WithServerName("data-worker"),
//	    sdk.WithServerApiToken(os.Getenv("SERVER_API_TOKEN")),
//	)
//
//	// Register job directly with the server
//	server.RegisterJob(&examples.DataProcessingJob{})
//
//	// Start handles everything: connection, registration, and blocking
//	server.Start()
//
// The job can be triggered with parameters:
//
//	{
//	    "source": "s3://bucket/data",
//	    "destination": "postgresql://db/table",
//	    "batch_size": 500,
//	    "validate": true,
//	    "dry_run": false
//	}
func ExampleDataProcessingJobUsage() {
	// This is a documentation example - see the function comment for usage
}
