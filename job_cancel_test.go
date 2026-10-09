package sdk

import (
	"sync"
	"testing"
	"time"

	"github.com/dibbla-agents/sdk-go/internal/state"
	"github.com/dibbla-agents/sdk-go/internal/types"
	"github.com/dibbla-agents/sdk-go/jobs"
)

// recordingComm captures what the worker sends to the server.
type recordingComm struct {
	mu     sync.Mutex
	events []*types.EventMessage
}

func (c *recordingComm) SendEvent(e *types.EventMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	return nil
}
func (c *recordingComm) ReceiveEvents() <-chan *types.EventMessage { return nil }
func (c *recordingComm) Close() error                              { return nil }
func (c *recordingComm) IsConnected() bool                         { return true }
func (c *recordingComm) WaitForConnection(time.Duration) error     { return nil }
func (c *recordingComm) SetOnReconnect(func())                     {}

func (c *recordingComm) lifecycle() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, e := range c.events {
		switch e.Event {
		case types.EventJobStarted, types.EventJobCompleted, types.EventJobFailed, types.EventJobCancelled:
			out = append(out, e.Event)
		}
	}
	return out
}

// funcJob adapts a function to jobs.JobHandler.
type funcJob struct{ run func(*jobs.JobContext) error }

func (j funcJob) Execute(ctx *jobs.JobContext) error { return j.run(ctx) }
func (funcJob) GetJobID() string                     { return "j" }
func (funcJob) GetJobName() string                   { return "J" }
func (funcJob) GetParameters() []jobs.JobParameter   { return nil }

func newJobTestServer(t *testing.T, job jobs.JobHandler) (*Server, *recordingComm) {
	t.Helper()
	comm := &recordingComm{}
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	s.globalState = &state.GlobalState{ServerName: "w", WorkflowComm: comm}
	s.RegisterJob(job)
	return s, comm
}

func waitRunning(t *testing.T, s *Server, runID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := s.runningJobs.Load(runID); ok {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job never started")
}

// A job that listens to its context stops on job_cancel and the run ends as
// cancelled — not failed, even though Execute returned ctx.Err().
func TestJobCancelStopsListeningJob(t *testing.T) {
	s, comm := newJobTestServer(t, funcJob{run: func(ctx *jobs.JobContext) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Millisecond):
			}
		}
	}})

	done := make(chan struct{})
	go func() { s.executeJob("run-1", "j", "J", nil); close(done) }()
	waitRunning(t, s, "run-1")

	s.handleJobCancel(&types.EventMessage{Event: types.EventJobCancel, Run: "run-1"})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("job did not stop after job_cancel")
	}
	got := comm.lifecycle()
	if len(got) != 2 || got[0] != types.EventJobStarted || got[1] != types.EventJobCancelled {
		t.Fatalf("lifecycle = %v, want [job_started job_cancelled]", got)
	}
	if _, ok := s.runningJobs.Load("run-1"); ok {
		t.Fatal("finished run still registered")
	}
}

// Backwards compatibility: a job that ignores the signal runs to the end and
// is still reported as cancelled, since a stop was asked for.
func TestJobCancelIgnoredByJobStillEndsCancelled(t *testing.T) {
	release := make(chan struct{})
	s, comm := newJobTestServer(t, funcJob{run: func(ctx *jobs.JobContext) error {
		<-release
		return nil
	}})
	done := make(chan struct{})
	go func() { s.executeJob("run-2", "j", "J", nil); close(done) }()
	waitRunning(t, s, "run-2")
	s.handleJobCancel(&types.EventMessage{Event: types.EventJobCancel, Run: "run-2"})
	close(release)
	<-done
	got := comm.lifecycle()
	if len(got) != 2 || got[1] != types.EventJobCancelled {
		t.Fatalf("lifecycle = %v, want job_cancelled last", got)
	}
}

// Without a cancel nothing changes: completed stays completed.
func TestJobWithoutCancelCompletes(t *testing.T) {
	s, comm := newJobTestServer(t, funcJob{run: func(ctx *jobs.JobContext) error { return nil }})
	s.executeJob("run-3", "j", "J", nil)
	got := comm.lifecycle()
	if len(got) != 2 || got[1] != types.EventJobCompleted {
		t.Fatalf("lifecycle = %v, want job_completed last", got)
	}
	// A cancel for a run that is no longer here is a no-op.
	s.handleJobCancel(&types.EventMessage{Event: types.EventJobCancel, Run: "run-3"})
}
