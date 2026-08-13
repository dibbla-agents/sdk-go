package communication

import (
	"fmt"
	"os"
	"strings"
)

// TokenProvider supplies the gRPC credential at every stream open. Because
// attemptConnection consults the provider on each (re)connect, a file-backed
// provider picks up kubelet token rotation with no extra machinery.
type TokenProvider interface {
	// Token returns the current credential ("" = none available).
	Token() (string, error)
	// Source is a short human-readable description for logs.
	Source() string
}

// StaticTokenProvider wraps a fixed API token (SERVER_API_TOKEN /
// WithServerApiToken) — the local-development and explicit-key path.
type StaticTokenProvider string

func (s StaticTokenProvider) Token() (string, error) { return string(s), nil }
func (s StaticTokenProvider) Source() string         { return "static api token" }

// DefaultIdentityTokenPath is where the Dibbla platform (deploy-api,
// DIB-202) mounts the projected, audience-scoped ServiceAccount token on
// tenant pods. The platform also sets DIBBLA_IDENTITY_TOKEN_FILE pointing
// here; the constant is the belt-and-braces fallback.
const DefaultIdentityTokenPath = "/var/run/secrets/dibbla/identity/token"

// IdentityTokenFileEnv is the env var the platform injects with the token
// file path.
const IdentityTokenFileEnv = "DIBBLA_IDENTITY_TOKEN_FILE"

// FileTokenProvider reads the credential from a file at every call —
// deliberately uncached, so the kubelet's rotation (~1h TTL, refreshed at
// ~80%) is observed at the next stream open. One small file read per
// (re)connect is negligible.
type FileTokenProvider struct {
	Path string
}

func (f *FileTokenProvider) Token() (string, error) {
	raw, err := os.ReadFile(f.Path)
	if err != nil {
		return "", fmt.Errorf("read identity token %s: %w", f.Path, err)
	}
	return strings.TrimSpace(string(raw)), nil
}

func (f *FileTokenProvider) Source() string { return "identity token file " + f.Path }

// DetectFileTokenProvider returns a FileTokenProvider for the workload
// identity token, if one is present: explicitPath (config/option) wins,
// then IdentityTokenFileEnv, then DefaultIdentityTokenPath. Returns nil
// when no readable token file exists — the caller falls back to "no
// credential" (warn) exactly as before workload identity existed.
func DetectFileTokenProvider(explicitPath string) *FileTokenProvider {
	candidates := []string{explicitPath, os.Getenv(IdentityTokenFileEnv), DefaultIdentityTokenPath}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return &FileTokenProvider{Path: p}
		}
	}
	return nil
}
