package communication

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dibbla-agents/sdk-go/internal/workflowsgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// authCapture records the authorization metadata each accepted stream
// presented, in order.
type authCapture struct {
	mu     sync.Mutex
	tokens []string
}

func (a *authCapture) record(stream grpc.BidiStreamingServer[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]) {
	md, _ := metadata.FromIncomingContext(stream.Context())
	tok := ""
	if v := md.Get("authorization"); len(v) > 0 {
		tok = v[0]
	}
	a.mu.Lock()
	a.tokens = append(a.tokens, tok)
	a.mu.Unlock()
}

func (a *authCapture) all() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.tokens...)
}

// TestFileProviderTokenRotationAcrossReconnects is the DIB-202 phase-3
// core test: the credential is re-read from the identity file at every
// stream open, so a kubelet rotation is presented on the next reconnect
// with no coordination.
func TestFileProviderTokenRotationAcrossReconnects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("first-token"), 0o600); err != nil {
		t.Fatal(err)
	}

	capture := &authCapture{}
	// Each stream: record the credential, consume registration, then die
	// with a retryable error so the client reconnects on the normal cadence.
	fake, gc := startFakeServer(t, func(stream grpc.BidiStreamingServer[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]) error {
		capture.record(stream)
		stream.Recv()
		return status.Error(codes.Unavailable, "server restarting")
	})
	_ = fake
	// Override the constructor's static token with the file provider, as
	// the factory does when SERVER_API_TOKEN is empty.
	gc.apiToken = ""
	gc.SetTokenProvider(&FileTokenProvider{Path: path})

	if err := gc.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer gc.Close()
	if err := gc.WaitForConnection(5 * time.Second); err != nil {
		t.Fatalf("never connected: %v", err)
	}

	// Rotate the file, then wait for at least one reconnect to present it.
	if err := os.WriteFile(path, []byte("second-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(15 * time.Second)
	for {
		toks := capture.all()
		if len(toks) >= 2 && toks[len(toks)-1] == "Bearer second-token" {
			if toks[0] != "Bearer first-token" {
				t.Fatalf("first connection presented %q, want first-token", toks[0])
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("rotated token never presented; captured %v", capture.all())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// TestUnauthenticatedWithRotatedCredentialRetriesPromptly: an auth
// rejection normally pins the backoff to the 5-minute holding pattern; but
// when the identity file has rotated since the rejected attempt, the next
// try uses the fresh token on the normal cadence instead.
func TestUnauthenticatedWithRotatedCredentialRetriesPromptly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("stale-token"), 0o600); err != nil {
		t.Fatal(err)
	}

	capture := &authCapture{}
	fake, gc := startFakeServer(t, func(stream grpc.BidiStreamingServer[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]) error {
		capture.record(stream)
		stream.Recv()
		return status.Error(codes.Unauthenticated, "Invalid or expired API token")
	})
	gc.apiToken = ""
	gc.SetTokenProvider(&FileTokenProvider{Path: path})
	gc.maxBackoff = 30 * time.Second // holding pattern >> test window

	if err := gc.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer gc.Close()
	if err := gc.WaitForConnection(5 * time.Second); err != nil {
		t.Fatalf("never connected: %v", err)
	}

	// Rotate immediately: the supervisor's rotation check should see the
	// fresh token and retry on the initial cadence (1s in this harness),
	// well inside the window — with maxBackoff pinning it would not.
	if err := os.WriteFile(path, []byte("fresh-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(10 * time.Second)
	for {
		for _, tok := range capture.all() {
			if tok == "Bearer fresh-token" {
				return
			}
		}
		select {
		case <-deadline:
			t.Fatalf("fresh token never presented within window (backoff wrongly pinned?); captured %v, streams=%d", capture.all(), fake.streams.Load())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// TestStaticTokenUnaffectedByProviderMachinery: the classic path — a static
// API token keeps the DIB-176 holding-pattern behavior on auth failure
// (credentialRotated is false for static credentials).
func TestStaticTokenUnaffectedByProviderMachinery(t *testing.T) {
	fake, gc := startFakeServer(t, rejectAfterRegistration)
	gc.maxBackoff = 30 * time.Second

	if err := gc.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer gc.Close()
	if err := gc.WaitForConnection(5 * time.Second); err != nil {
		t.Fatalf("never connected: %v", err)
	}

	time.Sleep(3 * time.Second)
	if n := fake.streams.Load(); n > 2 {
		t.Fatalf("static-token auth failure must hold at maxBackoff: %d streams in 3s", n)
	}
}
