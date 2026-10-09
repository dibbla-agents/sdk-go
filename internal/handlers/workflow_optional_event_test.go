package handlers

import (
	"testing"

	"github.com/dibbla-agents/sdk-go/internal/types"
)

// Job events carry no workflow; the inbound loop drops anything else that
// arrives without one. job_cancel missing here meant Stop never reached the
// job (found in DIB-1381's end-to-end run).
func TestJobEventsDoNotNeedAWorkflow(t *testing.T) {
	for _, e := range []string{types.EventJobTrigger, types.EventJobCancel} {
		if !isWorkflowOptionalEvent(e) {
			t.Errorf("%s is dropped when it has no workflow", e)
		}
	}
	if isWorkflowOptionalEvent(types.EventFunctionRequest) {
		t.Error("function_request must still require a workflow")
	}
}
