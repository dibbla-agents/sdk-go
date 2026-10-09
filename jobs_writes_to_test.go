package sdk

import (
	"testing"

	"github.com/dibbla-agents/sdk-go/jobs"
)

type plainJob struct{}

func (plainJob) Execute(*jobs.JobContext) error { return nil }
func (plainJob) GetJobID() string               { return "plain" }
func (plainJob) GetJobName() string             { return "Plain" }
func (plainJob) GetParameters() []jobs.JobParameter {
	return []jobs.JobParameter{{Name: "space", Type: "string"}}
}

type writingJob struct{ plainJob }

func (writingJob) WritesToDatabase() string { return " community_feedback " }

func TestJobRegistrationSchemaSaysWhichDatabaseAJobWritesTo(t *testing.T) {
	got := jobRegistrationSchema(writingJob{})
	if got["writes_to"] != "community_feedback" || got["name"] != "Plain" {
		t.Fatalf("schema=%v", got)
	}
	if _, ok := got["parameters"].(map[string]interface{})["space"]; !ok {
		t.Fatalf("parameters lost: %v", got["parameters"])
	}
	if _, ok := jobRegistrationSchema(plainJob{})["writes_to"]; ok {
		t.Fatal("a job that does not say must not send writes_to")
	}
}
