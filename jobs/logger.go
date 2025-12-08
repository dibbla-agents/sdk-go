package jobs

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dibbla-agents/sdk-go/internal/communication"
	"github.com/dibbla-agents/sdk-go/internal/types"
)

// Logger provides logging that sends events via gRPC
type Logger struct {
	comm       communication.WorkflowCommunicator
	serverName string
	runID      string
	jobID      string
	jobName    string
	taskName   string

	// Console output
	writer io.Writer

	// Progress tracking
	progressBar *ProgressBar
}

// NewLogger creates a new Logger instance
func NewLogger(comm communication.WorkflowCommunicator, serverName, runID, jobID, jobName string) *Logger {
	return &Logger{
		comm:       comm,
		serverName: serverName,
		runID:      runID,
		jobID:      jobID,
		jobName:    jobName,
		writer:     os.Stdout,
	}
}

// WithTask creates a task-scoped logger
func (l *Logger) WithTask(taskName string) *Logger {
	return &Logger{
		comm:       l.comm,
		serverName: l.serverName,
		runID:      l.runID,
		jobID:      l.jobID,
		jobName:    l.jobName,
		taskName:   taskName,
		writer:     l.writer,
	}
}

// WithWriter sets a custom writer for console output
func (l *Logger) WithWriter(w io.Writer) *Logger {
	l.writer = w
	return l
}

// sendEvent sends a job event via gRPC
func (l *Logger) sendEvent(eventType string, message string, extraMeta map[string]any) {
	meta := NewJobEventMeta(l.jobID, l.jobName)
	meta.TaskName = l.taskName

	metaMap := meta.ToMap()
	for k, v := range extraMeta {
		metaMap[k] = v
	}

	event := &types.EventMessage{
		Server:        l.serverName,
		Event:         eventType,
		Run:           l.runID,
		Text:          message,
		Meta:          &metaMap,
		CorrelationID: l.runID,
	}

	if err := l.comm.SendEvent(event); err != nil {
		fmt.Fprintf(l.writer, "[ERROR] Failed to send %s event: %v\n", eventType, err)
	}
}

// Info logs an info message
func (l *Logger) Info(message string) {
	l.printToConsole("INFO", message)
	l.sendEvent(types.EventLogMessage, message, map[string]any{
		"log_level": "info",
	})
}

// Warn logs a warning message
func (l *Logger) Warn(message string) {
	l.printToConsole("WARN", message)
	l.sendEvent(types.EventLogMessage, message, map[string]any{
		"log_level": "warn",
	})
}

// Warning is an alias for Warn
func (l *Logger) Warning(message string) {
	l.Warn(message)
}

// Error logs an error message
func (l *Logger) Error(message string) {
	l.printToConsole("ERROR", message)
	l.sendEvent(types.EventLogMessage, message, map[string]any{
		"log_level": "error",
	})
}

// Progress reports progress with known total
func (l *Logger) Progress(current, total int, message string) {
	l.sendEvent(types.EventProgressUpdate, message, map[string]any{
		"progress_current": current,
		"progress_total":   total,
	})

	// Update console progress bar
	if l.progressBar == nil {
		l.progressBar = NewProgressBar(total, l.writer)
	}
	l.progressBar.Update(current, message)
}

// ProgressIndeterminate reports progress without a known total
func (l *Logger) ProgressIndeterminate(current int, message string) {
	l.sendEvent(types.EventProgressUpdate, message, map[string]any{
		"progress_current": current,
		"progress_total":   0,
	})

	fmt.Fprintf(l.writer, "\r⏳ %s: %d processed...", message, current)
}

// CompleteProgress finishes the progress bar
func (l *Logger) CompleteProgress() {
	if l.progressBar != nil {
		l.progressBar.Finish()
		l.progressBar = nil
	} else {
		fmt.Fprintln(l.writer)
	}
}

// TaskStarted sends task_started event
func (l *Logger) TaskStarted(taskName string) {
	l.taskName = taskName
	l.printToConsole("TASK", fmt.Sprintf("Started: %s", taskName))
	l.sendEvent(types.EventTaskStarted, "", map[string]any{
		"status":    string(StatusInProgress),
		"task_name": taskName,
	})
}

// TaskCompleted sends task_completed event
func (l *Logger) TaskCompleted() {
	l.printToConsole("TASK", fmt.Sprintf("Completed: %s", l.taskName))
	l.sendEvent(types.EventTaskCompleted, "", map[string]any{
		"status":    string(StatusCompleted),
		"task_name": l.taskName,
	})
}

// TaskFailed sends task_failed event
func (l *Logger) TaskFailed(err error) {
	l.printToConsole("TASK", fmt.Sprintf("Failed: %s - %v", l.taskName, err))
	l.sendEvent(types.EventTaskFailed, "", map[string]any{
		"status":    string(StatusFailed),
		"task_name": l.taskName,
		"error":     err.Error(),
	})
}

// TaskSkipped sends task_skipped event
func (l *Logger) TaskSkipped(reason string) {
	l.printToConsole("TASK", fmt.Sprintf("Skipped: %s - %s", l.taskName, reason))
	l.sendEvent(types.EventTaskSkipped, reason, map[string]any{
		"status":    string(StatusSkipped),
		"task_name": l.taskName,
	})
}

// printToConsole outputs to local console
func (l *Logger) printToConsole(level, message string) {
	timestamp := time.Now().Format("15:04:05")
	taskInfo := ""
	if l.taskName != "" {
		taskInfo = fmt.Sprintf("[%s]", l.taskName)
	}
	fmt.Fprintf(l.writer, "%s [%s][%s]%s %s\n", timestamp, l.jobID, level, taskInfo, message)
}
