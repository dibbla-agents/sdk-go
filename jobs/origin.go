package jobs

import "net/http"

// Run-origin stamping (DIB-256).
//
// A job that calls a workflow mid-execution produces a `runs` row on the
// server that is indistinguishable from curl: nothing on the request says
// which pipeline task made the call. That is why a task could make 299
// workflow calls, have all 299 rejected, and still render green — the
// diagnosis existed, one table away, with no key to join on (DIB-235).
//
// The fix is three headers on the request the job already makes. This file
// produces them; it does not make the call, and nothing here changes how
// jobs are written.
//
// The contract mirrors workflow-server `endpoints/run_origin.go` exactly
// (DIB-235). Two properties of the server side drive the rules below:
//
//   - The stamp is validated all-or-nothing. A single over-long value makes
//     the server drop the whole stamp, losing the run from its parent's
//     totals entirely — so we never emit a value that would fail. An
//     over-long label degrades to no label rather than killing the stamp.
//   - An invalid or absent stamp is never an error. The run executes
//     normally with NULL origin columns; the pipeline view then renders
//     nothing for that task, because "no data" must never display as health.
const (
	OriginKindHeader  = "X-Dibbla-Origin-Kind"
	OriginIDHeader    = "X-Dibbla-Origin-Id"
	OriginLabelHeader = "X-Dibbla-Origin-Label"

	// OriginKindPipelineTask is the only kind the server allowlists today.
	OriginKindPipelineTask = "pipeline_task"

	// maxOriginValueLength matches MaxOriginValueLength server-side. Pipeline
	// run ids are ~65 chars ("pipeline-<id>-<unixnano>"), so only a pathological
	// task name can reach this.
	maxOriginValueLength = 256
)

// OriginHeaders returns the origin stamp for the workflow calls a job makes,
// or an empty map when there is nothing valid to stamp (nil context, or a run
// id that is empty or too long). The result is always safe to range over.
//
//	ctx.Logger.TaskStarted("GenerateSentiment")
//	origin := jobs.OriginHeaders(ctx)
//	// ... on each outgoing workflow request:
//	for k, v := range origin {
//	    req.Header.Set(k, v)
//	}
//
// The id is ctx.RunID — the run id workflow-server generated at trigger time
// and sent on the job_trigger event, which is the same string it stores as
// pipeline_runs.run_id. It is never worker-generated, so it always joins.
//
// The label is the current task name, read from the logger rather than passed
// in: it must equal the name the task_started event carried, or the server
// groups the runs under a label that matches no task and the result is
// invisible. Taking it from the same place TaskStarted got it makes them equal
// by construction instead of by the caller retyping the string correctly.
//
// Call it once per task, after TaskStarted and before any fan-out, then reuse
// the returned map. Logger.taskName is not mutex-guarded, so a job that starts
// another task while workers are still stamping would otherwise race.
func OriginHeaders(ctx *JobContext) map[string]string {
	if ctx == nil || ctx.RunID == "" || len(ctx.RunID) > maxOriginValueLength {
		return map[string]string{}
	}

	headers := map[string]string{
		OriginKindHeader: OriginKindPipelineTask,
		OriginIDHeader:   ctx.RunID,
	}

	// The label is display-only and the server accepts a stamp without one,
	// so an unusable task name costs the label, not the correlation.
	if ctx.Logger != nil {
		if label := ctx.Logger.taskName; label != "" && len(label) <= maxOriginValueLength {
			headers[OriginLabelHeader] = label
		}
	}

	return headers
}

// StampRequest applies OriginHeaders to an outgoing workflow request. It is
// the one-line form of the loop above, for jobs that build their own
// *http.Request:
//
//	req.Header.Set("Content-Type", "application/json")
//	jobs.StampRequest(req, ctx)
//
// A nil request or an unstampable context is a no-op — provenance is an audit
// tag, so it must never be a reason a job fails to make its call.
func StampRequest(req *http.Request, ctx *JobContext) {
	if req == nil {
		return
	}
	for k, v := range OriginHeaders(ctx) {
		req.Header.Set(k, v)
	}
}
