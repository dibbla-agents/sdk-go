package communication

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileTokenProviderReadsFreshOnEveryCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("tok-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := &FileTokenProvider{Path: path}
	if tok, err := p.Token(); err != nil || tok != "tok-1" {
		t.Fatalf("first read: %q %v", tok, err)
	}
	// Kubelet rotation = file content changes in place.
	if err := os.WriteFile(path, []byte("tok-2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if tok, err := p.Token(); err != nil || tok != "tok-2" {
		t.Fatalf("post-rotation read: %q %v (must not cache)", tok, err)
	}
}

func TestDetectFileTokenProviderPrecedence(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "explicit")
	envPath := filepath.Join(dir, "from-env")
	for _, p := range []string{explicit, envPath} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(IdentityTokenFileEnv, envPath)

	if got := DetectFileTokenProvider(explicit); got == nil || got.Path != explicit {
		t.Fatalf("explicit path must win, got %+v", got)
	}
	if got := DetectFileTokenProvider(""); got == nil || got.Path != envPath {
		t.Fatalf("env path must be used when no explicit path, got %+v", got)
	}

	t.Setenv(IdentityTokenFileEnv, "")
	// Default mount path won't exist on a dev machine → nil.
	if got := DetectFileTokenProvider(""); got != nil && got.Path != DefaultIdentityTokenPath {
		t.Fatalf("unexpected provider: %+v", got)
	}

	// Nonexistent explicit path falls through to nothing (env cleared).
	if got := DetectFileTokenProvider(filepath.Join(dir, "missing")); got != nil {
		t.Fatalf("missing file must not yield a provider, got %+v", got)
	}
}
