package sdk

import (
	"context"
	"testing"

	"github.com/dibbla-agents/sdk-go/internal/basefunction"
	"github.com/dibbla-agents/sdk-go/internal/types"
)

func eventWithMeta(meta map[string]any) *types.EventMessage {
	return &types.EventMessage{Meta: &meta}
}

func TestCallerFromContext_absentWhenPlatformAssertedNothing(t *testing.T) {
	// A call within a run no person triggered carries no asserted identity.
	// The contract is "no user", which a handler must be able to tell apart
	// from a user whose fields happen to be blank — otherwise it would
	// authorize an empty identity as if it were somebody.
	cases := map[string]*types.EventMessage{
		"nil event":      nil,
		"nil meta":       {},
		"empty meta":     eventWithMeta(map[string]any{}),
		"org but no who": eventWithMeta(map[string]any{types.MetaKeyAssertedOrgID: "org-1"}),
	}
	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			if c, ok := CallerFromContext(contextForEvent(msg)); ok {
				t.Fatalf("expected no caller, got %+v", c)
			}
		})
	}
}

func TestCallerFromContext_readsAssertedFields(t *testing.T) {
	ctx := contextForEvent(eventWithMeta(map[string]any{
		types.MetaKeyAssertedIdentity:  types.IdentityUserAuthenticated,
		types.MetaKeyAssertedUserID:    "user-42",
		types.MetaKeyAssertedUserEmail: "joakim@dibbla.com",
		types.MetaKeyAssertedUserName:  "Joakim Edlund",
		types.MetaKeyAssertedOrgID:     "org-1",
		types.MetaKeyAssertedOrgRole:   "owner",
	}))

	c, ok := CallerFromContext(ctx)
	if !ok {
		t.Fatal("expected a caller")
	}
	if !c.IsUser() {
		t.Error("expected IsUser to be true for an authenticated user")
	}
	for _, f := range []struct{ got, want, name string }{
		{c.UserID, "user-42", "UserID"},
		{c.Email, "joakim@dibbla.com", "Email"},
		{c.Name, "Joakim Edlund", "Name"},
		{c.OrgID, "org-1", "OrgID"},
		{c.OrgRole, "owner", "OrgRole"},
		{c.Identity, types.IdentityUserAuthenticated, "Identity"},
	} {
		if f.got != f.want {
			t.Errorf("%s = %q, want %q", f.name, f.got, f.want)
		}
	}
}

func TestCaller_IsUserRequiresBothIdentityAndSubject(t *testing.T) {
	// A future identity mode must not be mistaken for an interactive user, and
	// neither must an identity claim with nobody behind it.
	if (Caller{Identity: "some-future-mode", UserID: "user-42"}).IsUser() {
		t.Error("a non-user identity must not report IsUser")
	}
	if (Caller{Identity: IdentityUserAuthenticated}).IsUser() {
		t.Error("an authenticated identity with no user id must not report IsUser")
	}
}

func TestCallerFromContext_ignoresNonStringMeta(t *testing.T) {
	// Meta is map[string]any over the wire, so a malformed or future-typed
	// value must not panic a worker mid-invocation.
	ctx := contextForEvent(eventWithMeta(map[string]any{
		types.MetaKeyAssertedIdentity: types.IdentityUserAuthenticated,
		types.MetaKeyAssertedUserID:   "user-42",
		types.MetaKeyAssertedOrgRole:  42,
	}))
	c, ok := CallerFromContext(ctx)
	if !ok {
		t.Fatal("expected a caller")
	}
	if c.OrgRole != "" {
		t.Errorf("OrgRole = %q, want empty for a non-string value", c.OrgRole)
	}
}

func TestCallerFromContext_nilAndBareContext(t *testing.T) {
	if _, ok := CallerFromContext(nil); ok { //nolint:staticcheck // nil is the case under test
		t.Error("a nil context must not yield a caller")
	}
	if _, ok := CallerFromContext(context.Background()); ok {
		t.Error("a context with no caller must not yield one")
	}
}

type callerIn struct {
	Query string `json:"query"`
}
type callerOut struct {
	Seen string `json:"seen"`
}

// execute drives a built function the way the invoke handler does.
func execute(t *testing.T, fn basefunction.FunctionInterface, payload string, msg *types.EventMessage) (string, error) {
	t.Helper()
	in := []byte(payload)
	out, err := fn.Execute(&in, msg)
	if err != nil {
		return "", err
	}
	return string(*out), nil
}

func TestSimpleFunction_contextHandlerReceivesCaller(t *testing.T) {
	fn := NewSimpleFunction[callerIn, callerOut]("search", "1.0.0", "").
		WithContextHandler(func(ctx context.Context, in callerIn) (callerOut, error) {
			c, ok := CallerFromContext(ctx)
			if !ok {
				return callerOut{Seen: "anonymous"}, nil
			}
			return callerOut{Seen: c.Email}, nil
		}).
		Build(nil)

	got, err := execute(t, fn, `{"query":"x"}`, eventWithMeta(map[string]any{
		types.MetaKeyAssertedIdentity:  types.IdentityUserAuthenticated,
		types.MetaKeyAssertedUserID:    "user-42",
		types.MetaKeyAssertedUserEmail: "joakim@dibbla.com",
	}))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if want := `{"seen":"joakim@dibbla.com"}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestSimpleFunction_callerIsNotTakenFromInputs(t *testing.T) {
	// The whole point of carrying identity outside the payload: a caller that
	// shapes its inputs to look like an identity must not become that person.
	type spoofIn struct {
		AssertedUserEmail string `json:"asserted_user_email"`
		AssertedIdentity  string `json:"asserted_identity"`
	}
	fn := NewSimpleFunction[spoofIn, callerOut]("search", "1.0.0", "").
		WithContextHandler(func(ctx context.Context, in spoofIn) (callerOut, error) {
			if c, ok := CallerFromContext(ctx); ok {
				return callerOut{Seen: c.Email}, nil
			}
			return callerOut{Seen: "anonymous"}, nil
		}).
		Build(nil)

	got, err := execute(t, fn,
		`{"asserted_user_email":"admin@dibbla.com","asserted_identity":"user-authenticated"}`,
		eventWithMeta(map[string]any{}))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if want := `{"seen":"anonymous"}`; got != want {
		t.Errorf("inputs leaked into the caller: got %s, want %s", got, want)
	}
}

func TestSimpleFunction_handlerSettersAreExclusive(t *testing.T) {
	// Setting both handlers must be unambiguous rather than order-dependent,
	// in both directions.
	plain := func(callerIn) (callerOut, error) { return callerOut{Seen: "plain"}, nil }
	withCtx := func(context.Context, callerIn) (callerOut, error) { return callerOut{Seen: "ctx"}, nil }

	ctxWins := NewSimpleFunction[callerIn, callerOut]("f", "1.0.0", "").
		WithHandler(plain).WithContextHandler(withCtx).Build(nil)
	got, err := execute(t, ctxWins, `{}`, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if want := `{"seen":"ctx"}`; got != want {
		t.Errorf("last setter should win: got %s, want %s", got, want)
	}

	plainWins := NewSimpleFunction[callerIn, callerOut]("f", "1.0.0", "").
		WithContextHandler(withCtx).WithHandler(plain).Build(nil)
	if got, err = execute(t, plainWins, `{}`, nil); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if want := `{"seen":"plain"}`; got != want {
		t.Errorf("last setter should win: got %s, want %s", got, want)
	}
}

func TestSimpleFunction_missingHandlerIsAnError(t *testing.T) {
	// A wiring mistake should name the function, not surface as a nil
	// dereference inside the worker's dispatch goroutine.
	fn := NewSimpleFunction[callerIn, callerOut]("forgot", "2.0.0", "").Build(nil)
	_, err := execute(t, fn, `{}`, nil)
	if err == nil {
		t.Fatal("expected an error for a function registered without a handler")
	}
	for _, want := range []string{"forgot", "2.0.0", "handler"} {
		if !contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// A call within a workflow run a person triggered (DIB-1276): the person and
// the run are read, ok is true, IsWorkflowRunForUser is true and IsUser is
// false — the person did not call the function directly.
func TestCallerFromContext_workflowRunForAPerson(t *testing.T) {
	c, ok := CallerFromContext(contextForEvent(eventWithMeta(map[string]any{
		types.MetaKeyAssertedIdentity:  types.IdentityWorkflowRunUser,
		types.MetaKeyAssertedUserID:    "u-1",
		types.MetaKeyAssertedUserEmail: "anna@example.com",
		types.MetaKeyAssertedUserName:  "Anna",
		types.MetaKeyAssertedOrgID:     "org-1",
		types.MetaKeyAssertedOrgRole:   "developer",
		types.MetaKeyAssertedRunID:     "run-1",
		types.MetaKeyAssertedWorkflow:  "meeting-summary",
		types.MetaKeyAssertedTrigger:   "user",
	})))
	if !ok {
		t.Fatal("an asserted workflow-run identity is a caller")
	}
	want := Caller{UserID: "u-1", Email: "anna@example.com", Name: "Anna", OrgID: "org-1", OrgRole: "developer",
		Identity: IdentityWorkflowRunUser, RunID: "run-1", Workflow: "meeting-summary", Trigger: "user"}
	if c != want {
		t.Fatalf("caller = %+v\nwant %+v", c, want)
	}
	if c.IsUser() {
		t.Fatal("a workflow run is not a user calling directly")
	}
	if !c.IsWorkflowRunForUser() {
		t.Fatal("IsWorkflowRunForUser")
	}
}

// IsWorkflowRunForUser needs the identity, a person and a run; nothing else
// qualifies, and a direct user is not a workflow run.
func TestCaller_IsWorkflowRunForUserRequiresIdentityPersonAndRun(t *testing.T) {
	for name, c := range map[string]Caller{
		"no person":      {Identity: IdentityWorkflowRunUser, RunID: "run-1"},
		"no run":         {Identity: IdentityWorkflowRunUser, UserID: "u-1"},
		"direct user":    {Identity: IdentityUserAuthenticated, UserID: "u-1", RunID: "run-1"},
		"api key":        {Identity: IdentityAPIKey, UserID: "u-1", RunID: "run-1"},
		"future mode":    {Identity: "some-future-mode", UserID: "u-1", RunID: "run-1"},
		"nothing at all": {},
	} {
		if c.IsWorkflowRunForUser() {
			t.Errorf("%s: IsWorkflowRunForUser is true", name)
		}
	}
}

// A function's inputs cannot make a call look like a workflow run.
func TestSimpleFunction_workflowRunIsNotTakenFromInputs(t *testing.T) {
	type spoofIn struct {
		AssertedIdentity string `json:"asserted_identity"`
		AssertedRunID    string `json:"asserted_run_id"`
		AssertedUserID   string `json:"asserted_user_id"`
	}
	fn := NewSimpleFunction[spoofIn, callerOut]("search", "1.0.0", "").
		WithContextHandler(func(ctx context.Context, in spoofIn) (callerOut, error) {
			if c, ok := CallerFromContext(ctx); ok && c.IsWorkflowRunForUser() {
				return callerOut{Seen: c.UserID}, nil
			}
			return callerOut{Seen: "anonymous"}, nil
		}).
		Build(nil)
	got, err := execute(t, fn, `{"asserted_identity":"workflow-run-user","asserted_run_id":"run-1","asserted_user_id":"u-1"}`, eventWithMeta(map[string]any{}))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if want := `{"seen":"anonymous"}`; got != want {
		t.Errorf("inputs became a workflow-run caller: got %s, want %s", got, want)
	}
}
