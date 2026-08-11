package communication

import (
	"context"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dibbla-agents/sdk-go/internal/types"
	"github.com/dibbla-agents/sdk-go/internal/workflowsgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakeEventServer is a controllable EventService implementation.
// streamBehavior decides what each accepted stream does.
type fakeEventServer struct {
	workflowsgrpc.UnimplementedEventServiceServer
	streams        atomic.Int64 // number of streams ever accepted
	streamBehavior func(stream grpc.BidiStreamingServer[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]) error
}

func (f *fakeEventServer) Events(stream grpc.BidiStreamingServer[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]) error {
	f.streams.Add(1)
	return f.streamBehavior(stream)
}

// startFakeServer runs a fake EventService over an in-memory listener and
// returns the server plus a communicator wired to dial it.
func startFakeServer(t *testing.T, behavior func(stream grpc.BidiStreamingServer[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]) error) (*fakeEventServer, *GrpcCommunicator) {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	fake := &fakeEventServer{streamBehavior: behavior}
	srv := grpc.NewServer()
	workflowsgrpc.RegisterEventServiceServer(srv, fake)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	// "localhost:0" disables TLS; the bufconn dialer ignores the address.
	gc := NewGrpcCommunicatorWithTLS("localhost:0", "test-server", "test-token", 10, 1, 1, 0, false)
	gc.dialOpts = []grpc.DialOption{grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	})}
	return fake, gc
}

// rejectAfterRegistration mimics the DIB-176 server: the stream is accepted,
// registration is received, then the stream dies with Unauthenticated —
// exactly the connect-succeeds-then-Recv-fails shape of the incident.
func rejectAfterRegistration(stream grpc.BidiStreamingServer[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]) error {
	stream.Recv() // consume registration
	return status.Error(codes.Unauthenticated, "Invalid or expired API token")
}

// TestUnauthenticatedStreamDoesNotSpin is the DIB-176 regression test: a
// token rejected on first Recv must NOT cause a zero-delay reconnect loop.
// Before the fix this produced ~172 reconnects/second (hundreds in this
// window); with classification + backoff we allow at most the initial attempt
// plus one retry.
func TestUnauthenticatedStreamDoesNotSpin(t *testing.T) {
	fake, gc := startFakeServer(t, rejectAfterRegistration)

	if err := gc.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer gc.Close()

	if err := gc.WaitForConnection(5 * time.Second); err != nil {
		t.Fatalf("never connected: %v", err)
	}

	time.Sleep(3 * time.Second)

	if n := fake.streams.Load(); n > 2 {
		t.Fatalf("reconnect spin: %d streams accepted in 3s, want <= 2 (pre-fix: ~500)", n)
	}
}

// TestAuthFailureEscalatesToMaxBackoff verifies the classification path: a
// non-retryable stream error must jump straight to maxBackoff instead of
// walking the exponential ladder.
func TestAuthFailureEscalatesToMaxBackoff(t *testing.T) {
	fake, gc := startFakeServer(t, rejectAfterRegistration)
	gc.maxBackoff = 10 * time.Second // still >> the 1s initial interval

	if err := gc.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer gc.Close()

	if err := gc.WaitForConnection(5 * time.Second); err != nil {
		t.Fatalf("never connected: %v", err)
	}

	// With reconnectIntervalSec=1 the exponential ladder alone would allow a
	// second stream within ~1s. With auth classification the next attempt is
	// >= maxBackoff/2 (=5s with jitter) away, so after 3s we must still be at
	// exactly 1 stream.
	time.Sleep(3 * time.Second)
	if n := fake.streams.Load(); n != 1 {
		t.Fatalf("auth failure retried too fast: %d streams in 3s, want exactly 1", n)
	}
}

// TestNoGoroutineLeakAcrossReconnects is the leak regression test: repeated
// disconnects must not accumulate goroutines (pre-fix: +2 goroutines and +2
// tickers per reconnect, ~12 KiB each → OOM every 33 min in production).
func TestNoGoroutineLeakAcrossReconnects(t *testing.T) {
	release := make(chan struct{}, 64)
	// Each stream stays open until the test releases it, then dies with a
	// retryable (non-auth) error so reconnection continues at the 1s interval.
	fake, gc := startFakeServer(t, func(stream grpc.BidiStreamingServer[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]) error {
		stream.Recv()
		<-release
		return status.Error(codes.Unavailable, "server going away")
	})
	gc.pingIntervalSec = 1                  // exercise pingLoop lifecycle too
	gc.healthyResetAfter = time.Millisecond // each ~100ms stint counts as healthy → constant 1s retry interval

	if err := gc.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer gc.Close()

	if err := gc.WaitForConnection(5 * time.Second); err != nil {
		t.Fatalf("never connected: %v", err)
	}

	waitForStreams := func(n int64) {
		deadline := time.Now().Add(10 * time.Second)
		for fake.streams.Load() < n {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for stream #%d (have %d)", n, fake.streams.Load())
			}
			time.Sleep(20 * time.Millisecond)
		}
	}

	// Let the first connection settle, then measure the baseline.
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	baseline := runtime.NumGoroutine()

	// Force 5 full disconnect→reconnect cycles.
	const cycles = 5
	for i := int64(2); i <= cycles+1; i++ {
		release <- struct{}{}
		waitForStreams(i)
		time.Sleep(100 * time.Millisecond) // let per-connection goroutines start
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()

	// Identical connected state before and after; allow scheduling slack but
	// catch the pre-fix behavior (+2 per cycle = +10 here).
	if after > baseline+3 {
		t.Fatalf("goroutine leak: baseline %d, after %d cycles %d", baseline, cycles, after)
	}
}

// TestBackoffResetsAfterHealthyConnection verifies that a connection that
// stays up past healthyResetAfter earns a reset to the initial interval —
// while a connect-then-die loop keeps escalating.
func TestBackoffResetsAfterHealthyConnection(t *testing.T) {
	release := make(chan struct{}, 64)
	fake, gc := startFakeServer(t, func(stream grpc.BidiStreamingServer[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]) error {
		stream.Recv()
		<-release
		return status.Error(codes.Unavailable, "server going away")
	})
	gc.healthyResetAfter = 300 * time.Millisecond
	gc.maxBackoff = 30 * time.Second

	if err := gc.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer gc.Close()

	if err := gc.WaitForConnection(5 * time.Second); err != nil {
		t.Fatalf("never connected: %v", err)
	}

	// Stay healthy past the reset threshold, then kill the connection.
	time.Sleep(500 * time.Millisecond)
	release <- struct{}{}

	// After a healthy stint the retry delay is the initial interval (1s,
	// jittered to [0.5s, 1s]) — so a reconnect must appear within ~2s.
	deadline := time.Now().Add(2 * time.Second)
	for fake.streams.Load() < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("backoff did not reset: no reconnect within 2s of healthy disconnect")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestCloseIsCleanWhileConnected verifies Close() tears everything down
// without panics (send on closed channel) and terminates the consumer range.
func TestCloseIsCleanWhileConnected(t *testing.T) {
	block := make(chan struct{})
	_, gc := startFakeServer(t, func(stream grpc.BidiStreamingServer[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]) error {
		stream.Recv()
		<-block
		return nil
	})
	defer close(block)
	gc.pingIntervalSec = 1

	if err := gc.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := gc.WaitForConnection(5 * time.Second); err != nil {
		t.Fatalf("never connected: %v", err)
	}

	consumerDone := make(chan struct{})
	go func() {
		for range gc.ReceiveEvents() {
		}
		close(consumerDone)
	}()

	closed := make(chan struct{})
	go func() {
		gc.Close()
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close() did not return within 5s")
	}
	select {
	case <-consumerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("incomingEvents was not closed for the consumer")
	}
}

// TestConcurrentSendEventIsSerialised hammers SendEvent from many goroutines.
// grpc-go forbids concurrent SendMsg on one stream; pre-fix SendEvent held
// only an RLock (no mutual exclusion). Run with -race.
func TestConcurrentSendEventIsSerialised(t *testing.T) {
	block := make(chan struct{})
	_, gc := startFakeServer(t, func(stream grpc.BidiStreamingServer[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]) error {
		for { // consume everything until the test ends
			if _, err := stream.Recv(); err != nil {
				<-block
				return nil
			}
		}
	})
	defer close(block)

	if err := gc.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer gc.Close()
	if err := gc.WaitForConnection(5 * time.Second); err != nil {
		t.Fatalf("never connected: %v", err)
	}

	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				gc.SendEvent(&types.EventMessage{Server: "test-server", Event: "test_event", Text: "x"})
			}
		}()
	}
	wg.Wait()
}
