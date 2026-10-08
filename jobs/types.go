package jobs

import (
	"time"
)

// JobStatus represents the status of a job or task
type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusInProgress JobStatus = "in_progress"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
	StatusSkipped    JobStatus = "skipped"
)

// JobParameter describes a parameter for a job.
//
// Name, Type, Required and Default are what every SDK version sends. The
// optional fields below (DIB-1379) let the console render the right input and
// refuse a wrong value before a pipeline is saved, instead of the job failing
// when it runs. A workflow server that doesn't know them ignores them.
type JobParameter struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Required bool        `json:"required"`
	Default  interface{} `json:"default,omitempty"`

	// Description says what the parameter is for, shown under its field.
	Description string `json:"description,omitempty"`
	// Options, when set, are the only allowed values (shown as a select).
	Options []interface{} `json:"options,omitempty"`
	// Format refines a string parameter: one of the Format* constants.
	Format string `json:"format,omitempty"`
	// Min and Max bound a number parameter, inclusive.
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// Formats a string parameter can declare.
const (
	FormatDate     = "date"     // YYYY-MM-DD
	FormatDateTime = "datetime" // RFC 3339, e.g. 2026-10-01T06:00:00Z
	FormatURL      = "url"      // http(s)://…
	FormatEmail    = "email"
	FormatList     = "list" // a list of strings; the job receives []interface{}
	FormatJSON     = "json" // any JSON value; the job receives it decoded
)

// Bound returns a pointer to v, for JobParameter.Min and Max.
func Bound(v float64) *float64 { return &v }

// Schema is the parameter as the job registration sends it: the keys the
// workflow server reads, with the optional ones only when set.
func (p JobParameter) Schema() map[string]interface{} {
	m := map[string]interface{}{
		"type":     p.Type,
		"required": p.Required,
		"default":  p.Default,
	}
	if p.Description != "" {
		m["description"] = p.Description
	}
	if len(p.Options) > 0 {
		m["options"] = p.Options
	}
	if p.Format != "" {
		m["format"] = p.Format
	}
	if p.Min != nil {
		m["min"] = *p.Min
	}
	if p.Max != nil {
		m["max"] = *p.Max
	}
	return m
}

// JobHandler is the interface that job implementations must satisfy
type JobHandler interface {
	Execute(ctx *JobContext) error
	GetJobID() string
	GetJobName() string
	GetParameters() []JobParameter
}

// JobEventMeta defines the structured metadata for job events
// This schema is shared with the server
type JobEventMeta struct {
	JobID           string `json:"job_id"`
	JobName         string `json:"job_name"`
	Timestamp       string `json:"timestamp"`
	TaskName        string `json:"task_name,omitempty"`
	Status          string `json:"status,omitempty"`
	LogLevel        string `json:"log_level,omitempty"`
	Error           string `json:"error,omitempty"`
	ProgressCurrent int    `json:"progress_current,omitempty"`
	ProgressTotal   int    `json:"progress_total,omitempty"`
}

// ToMap converts JobEventMeta to map[string]any for EventMessage.Meta
func (m *JobEventMeta) ToMap() map[string]any {
	result := map[string]any{
		"job_id":    m.JobID,
		"job_name":  m.JobName,
		"timestamp": m.Timestamp,
	}
	if m.TaskName != "" {
		result["task_name"] = m.TaskName
	}
	if m.Status != "" {
		result["status"] = m.Status
	}
	if m.LogLevel != "" {
		result["log_level"] = m.LogLevel
	}
	if m.Error != "" {
		result["error"] = m.Error
	}
	if m.ProgressTotal > 0 {
		result["progress_current"] = m.ProgressCurrent
		result["progress_total"] = m.ProgressTotal
	}
	return result
}

// NewJobEventMeta creates a new JobEventMeta with current timestamp
func NewJobEventMeta(jobID, jobName string) *JobEventMeta {
	return &JobEventMeta{
		JobID:     jobID,
		JobName:   jobName,
		Timestamp: time.Now().Format(time.RFC3339Nano),
	}
}

// ParseJobEventMeta extracts JobEventMeta from EventMessage.Meta
func ParseJobEventMeta(m *map[string]any) *JobEventMeta {
	if m == nil {
		return &JobEventMeta{}
	}
	meta := &JobEventMeta{}
	mm := *m

	if v, ok := mm["job_id"].(string); ok {
		meta.JobID = v
	}
	if v, ok := mm["job_name"].(string); ok {
		meta.JobName = v
	}
	if v, ok := mm["timestamp"].(string); ok {
		meta.Timestamp = v
	}
	if v, ok := mm["task_name"].(string); ok {
		meta.TaskName = v
	}
	if v, ok := mm["status"].(string); ok {
		meta.Status = v
	}
	if v, ok := mm["log_level"].(string); ok {
		meta.LogLevel = v
	}
	if v, ok := mm["error"].(string); ok {
		meta.Error = v
	}
	// Handle both float64 (JSON) and int
	if v, ok := mm["progress_current"].(float64); ok {
		meta.ProgressCurrent = int(v)
	} else if v, ok := mm["progress_current"].(int); ok {
		meta.ProgressCurrent = v
	}
	if v, ok := mm["progress_total"].(float64); ok {
		meta.ProgressTotal = int(v)
	} else if v, ok := mm["progress_total"].(int); ok {
		meta.ProgressTotal = v
	}

	return meta
}
