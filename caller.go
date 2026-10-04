package sdk

import (
	"context"

	"github.com/dibbla-agents/sdk-go/internal/types"
)

// Caller is the platform-verified identity behind a function invocation.
//
// WHY THIS EXISTS. A function that is exposed as a directly-callable tool runs
// on behalf of a specific person, and anything it reads or writes should be
// authorized for that person — not for whoever deployed the worker. Until now
// a worker had no way to learn who was calling: SimpleFunction handlers get
// only the decoded input, and the richer Function handler reaches the caller
// through internal/types, which Go's internal rule puts out of reach of every
// module except this one. Identity that only internal code can read is
// identity an SDK consumer does not have.
//
// WHY IT IS NOT AN INPUT FIELD. The obvious shortcut — let the caller pass
// its own email — makes every authorization decision advisory, because the
// caller writes both the question and the answer. These values are set by the
// platform from the validated principal and travel outside the payload, so a
// caller cannot assert an identity it does not hold. This mirrors the rule the
// platform's own toolsets follow, where identity comes from the token and
// never from tool arguments.
type Caller struct {
	// UserID is the stable platform user id. Prefer it as a database key;
	// an email can be reassigned, this cannot.
	UserID string
	// Email and Name describe the person, for display and for matching
	// against your own user records.
	Email string
	Name  string
	// OrgID is the organization the call was made in. A worker serving one
	// organization should reject anything else rather than assume.
	OrgID   string
	OrgRole string
	// Identity records how the caller was established, e.g.
	// IdentityUserAuthenticated. Compare against it rather than assuming any
	// populated Caller is an interactive user — future modes will land here.
	Identity string
	// RunID, Workflow and Trigger are set for a call made within a workflow
	// run (IdentityWorkflowRunUser): the run, its workflow's name, and what
	// started it ("user", "mcp", or an origin such as "app").
	RunID    string
	Workflow string
	Trigger  string
}

// IdentityUserAuthenticated means a signed-in user called the function
// directly (platform MCP, API or CLI) and the Caller describes that person.
const IdentityUserAuthenticated = types.IdentityUserAuthenticated

// IdentityAPIKey means a machine credential called the function directly; no
// person is behind the call.
const IdentityAPIKey = types.IdentityAPIKey

// IdentityWorkflowRunUser means the call was made within a workflow run that a
// person triggered: the Caller describes that person and the run. The person
// did not call the function themselves — a graph they ran did — so IsUser is
// false; a function decides for itself whether to act for them
// (IsWorkflowRunForUser).
const IdentityWorkflowRunUser = types.IdentityWorkflowRunUser

// IsUser reports whether the call carries an authenticated end user calling
// directly. A function that reads per-user data should refuse when this is
// false rather than falling back to a default identity. A workflow run is not
// a user calling directly: see IsWorkflowRunForUser.
func (c Caller) IsUser() bool {
	return c.Identity == IdentityUserAuthenticated && c.UserID != ""
}

// IsWorkflowRunForUser reports whether the call was made within a workflow
// run that the platform says a person triggered. Acting for that person is a
// choice each function makes — reading may be fine where writing is not —
// so this is deliberately separate from IsUser, and nothing here combines
// the two.
func (c Caller) IsWorkflowRunForUser() bool {
	return c.Identity == IdentityWorkflowRunUser && c.UserID != "" && c.RunID != ""
}

type callerContextKey struct{}

// CallerFromContext returns the verified caller for this invocation.
//
// ok is false when the platform asserted no identity — a call within a
// workflow run no person triggered, for instance. Treat that as "no user",
// never as "some default user": a handler that reads personal data should
// return an error, and one that does not need identity can ignore the second
// return. ok is true for every asserted identity, including ones that are not
// a person calling directly (IdentityWorkflowRunUser, IdentityAPIKey): check
// IsUser or IsWorkflowRunForUser before acting for a person.
func CallerFromContext(ctx context.Context) (Caller, bool) {
	if ctx == nil {
		return Caller{}, false
	}
	c, ok := ctx.Value(callerContextKey{}).(Caller)
	if !ok || c.Identity == "" {
		return Caller{}, false
	}
	return c, true
}

// ContextWithCaller returns ctx carrying c. Exported for tests and for
// consumers driving a handler directly; the SDK installs the caller itself on
// every invocation, so handlers do not need to call this.
func ContextWithCaller(ctx context.Context, c Caller) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, callerContextKey{}, c)
}

// callerFromEvent reads the asserted identity the platform placed on an
// invocation. An event without asserted fields yields the zero Caller, which
// CallerFromContext reports as absent.
func callerFromEvent(msg *types.EventMessage) Caller {
	if msg == nil || msg.Meta == nil {
		return Caller{}
	}
	meta := *msg.Meta
	str := func(key string) string {
		s, _ := meta[key].(string)
		return s
	}
	return Caller{
		UserID:   str(types.MetaKeyAssertedUserID),
		Email:    str(types.MetaKeyAssertedUserEmail),
		Name:     str(types.MetaKeyAssertedUserName),
		OrgID:    str(types.MetaKeyAssertedOrgID),
		OrgRole:  str(types.MetaKeyAssertedOrgRole),
		Identity: str(types.MetaKeyAssertedIdentity),
		RunID:    str(types.MetaKeyAssertedRunID),
		Workflow: str(types.MetaKeyAssertedWorkflow),
		Trigger:  str(types.MetaKeyAssertedTrigger),
	}
}

// contextForEvent builds the context handed to a context-shaped handler.
//
// The base is context.Background(): the invocation path does not carry a
// cancellable context today, so this context is never cancelled and has no
// deadline. Handlers should still accept and propagate it — when the platform
// starts sending a deadline, it arrives here and callers that already thread
// ctx through their I/O get cancellation without changing a signature.
func contextForEvent(msg *types.EventMessage) context.Context {
	return ContextWithCaller(context.Background(), callerFromEvent(msg))
}
