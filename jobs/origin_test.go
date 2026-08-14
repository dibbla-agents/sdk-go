package jobs

import (
	"net/http"
	"strings"
	"testing"
)

// ctxWithTask builds a JobContext whose logger reports taskName, without
// needing a communicator (nothing here sends events).
func ctxWithTask(runID, taskName string) *JobContext {
	return &JobContext{
		RunID:  runID,
		JobID:  "community-feedback",
		Logger: &Logger{taskName: taskName},
	}
}

func TestOriginHeadersStampsPipelineTask(t *testing.T) {
	got := OriginHeaders(ctxWithTask("pipeline-abc-1755180000000000000", "GenerateSentiment"))

	want := map[string]string{
		OriginKindHeader:  "pipeline_task",
		OriginIDHeader:    "pipeline-abc-1755180000000000000",
		OriginLabelHeader: "GenerateSentiment",
	}
	if len(got) != len(want) {
		t.Fatalf("header count = %d, want %d (%v)", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

// The header names and kind value are a wire contract with workflow-server
// (endpoints/run_origin.go). A rename on either side silently stops the join,
// so pin the literals rather than the constants.
func TestOriginHeaderContractMatchesServer(t *testing.T) {
	if OriginKindHeader != "X-Dibbla-Origin-Kind" {
		t.Errorf("kind header = %q", OriginKindHeader)
	}
	if OriginIDHeader != "X-Dibbla-Origin-Id" {
		t.Errorf("id header = %q", OriginIDHeader)
	}
	if OriginLabelHeader != "X-Dibbla-Origin-Label" {
		t.Errorf("label header = %q", OriginLabelHeader)
	}
	if OriginKindPipelineTask != "pipeline_task" {
		t.Errorf("kind value = %q", OriginKindPipelineTask)
	}
}

func TestOriginHeadersOmitsLabelWhenNoTaskStarted(t *testing.T) {
	got := OriginHeaders(ctxWithTask("pipeline-abc-1", ""))

	if _, ok := got[OriginLabelHeader]; ok {
		t.Errorf("label header present for unnamed task: %v", got)
	}
	// The correlation must survive: the server only requires kind and id.
	if got[OriginIDHeader] != "pipeline-abc-1" {
		t.Errorf("id = %q, want the run id", got[OriginIDHeader])
	}
}

// An over-long label would make the server reject the stamp whole, so it
// degrades to no label rather than costing the run its parent.
func TestOriginHeadersDropsOversizedLabelKeepsStamp(t *testing.T) {
	got := OriginHeaders(ctxWithTask("pipeline-abc-1", strings.Repeat("t", maxOriginValueLength+1)))

	if _, ok := got[OriginLabelHeader]; ok {
		t.Errorf("oversized label was sent: %v", got)
	}
	if got[OriginKindHeader] != OriginKindPipelineTask || got[OriginIDHeader] != "pipeline-abc-1" {
		t.Errorf("stamp lost with oversized label: %v", got)
	}
}

func TestOriginHeadersAcceptsLabelAtLimit(t *testing.T) {
	label := strings.Repeat("t", maxOriginValueLength)
	got := OriginHeaders(ctxWithTask("pipeline-abc-1", label))

	if got[OriginLabelHeader] != label {
		t.Errorf("label at the limit was dropped")
	}
}

func TestOriginHeadersEmptyWhenNothingToStamp(t *testing.T) {
	cases := map[string]*JobContext{
		"nil context":  nil,
		"no run id":    ctxWithTask("", "GenerateSentiment"),
		"nil logger":   {RunID: "pipeline-abc-1", Logger: nil},
		"oversized id": ctxWithTask(strings.Repeat("r", maxOriginValueLength+1), "GenerateSentiment"),
	}

	for name, ctx := range cases {
		got := OriginHeaders(ctx)
		if got == nil {
			t.Errorf("%s: returned nil, want a rangeable map", name)
		}
		if name == "nil logger" {
			// A logger-less context still correlates; it just has no label.
			if got[OriginIDHeader] != "pipeline-abc-1" {
				t.Errorf("%s: lost the stamp (%v)", name, got)
			}
			continue
		}
		if len(got) != 0 {
			t.Errorf("%s: stamped %v, want nothing", name, got)
		}
	}
}

func TestStampRequestSetsHeadersOnRequest(t *testing.T) {
	req, err := http.NewRequest("POST", "https://example.invalid/api/wf/execute/agent/prod", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}

	StampRequest(req, ctxWithTask("pipeline-abc-1", "GenerateSentiment"))

	if got := req.Header.Get(OriginIDHeader); got != "pipeline-abc-1" {
		t.Errorf("%s = %q", OriginIDHeader, got)
	}
	if got := req.Header.Get(OriginLabelHeader); got != "GenerateSentiment" {
		t.Errorf("%s = %q", OriginLabelHeader, got)
	}
	if got := req.Header.Get(OriginKindHeader); got != "pipeline_task" {
		t.Errorf("%s = %q", OriginKindHeader, got)
	}
}

// Stamping is an audit tag: it must never be the reason a job fails to make
// its call, and it must not disturb headers the job already set.
func TestStampRequestIsNoOpWhenUnstampable(t *testing.T) {
	req, err := http.NewRequest("POST", "https://example.invalid/execute", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	StampRequest(req, ctxWithTask("", ""))
	StampRequest(nil, ctxWithTask("pipeline-abc-1", "GenerateSentiment")) // must not panic

	if req.Header.Get(OriginKindHeader) != "" {
		t.Errorf("stamped an unstampable context: %v", req.Header)
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("disturbed the caller's headers: %v", req.Header)
	}
}
