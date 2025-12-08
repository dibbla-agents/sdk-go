package jobs

import (
	"fmt"
	"math/rand"
	"time"
)

// GenerateRunID generates a unique run ID
// Format: run_{timestamp}_{random}
func GenerateRunID() string {
	now := time.Now()
	timestamp := now.Format("20060102_150405")
	random := fmt.Sprintf("%04x", rand.Intn(0xFFFF))
	return fmt.Sprintf("run_%s_%s", timestamp, random)
}

// GenerateJobRunID generates a run ID prefixed with job ID
// Format: {job_id}_run_{timestamp}_{random}
func GenerateJobRunID(jobID string) string {
	now := time.Now()
	timestamp := now.Format("20060102_150405")
	random := fmt.Sprintf("%04x", rand.Intn(0xFFFF))
	return fmt.Sprintf("%s_run_%s_%s", jobID, timestamp, random)
}
