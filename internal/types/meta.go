package types

// Meta keys the platform sets on a function invocation. They are mirrored
// from core/workflow-server/types/eventmeta.go and must stay byte-identical
// with it — the two sides agree by string, not by shared code.
//
// These values are set by the platform from the validated principal and are
// never read from a function's inputs. A worker must therefore treat them as
// the only trustworthy statement of who is calling; anything arriving inside
// the payload is caller-controlled.
const (
	MetaKeyAssertedOrgID    = "asserted_org_id"
	MetaKeyAssertedIdentity = "asserted_identity"

	// Added for the direct-invoke path so a function can authorize per user
	// rather than only per organization.
	MetaKeyAssertedUserID    = "asserted_user_id"
	MetaKeyAssertedUserEmail = "asserted_user_email"
	MetaKeyAssertedUserName  = "asserted_user_name"
	MetaKeyAssertedOrgRole   = "asserted_org_role"
)

// Values MetaKeyAssertedIdentity can take.
const (
	// IdentityUserAuthenticated: a signed-in user called the function
	// directly (platform MCP, API or CLI). The asserted user fields describe
	// that person.
	IdentityUserAuthenticated = "user-authenticated"
)
