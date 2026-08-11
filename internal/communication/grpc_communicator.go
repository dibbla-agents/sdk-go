package communication

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/dibbla-agents/sdk-go/internal/types"
	"github.com/dibbla-agents/sdk-go/internal/workflowsgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Reconnection defaults. maxBackoff also serves as the holding pattern for
// non-retryable errors (e.g. an invalid API token): the client keeps retrying
// at this slow cadence so it self-heals once the cause is fixed, without
// hammering the server in the meantime.
const (
	defaultMaxBackoff        = 5 * time.Minute
	defaultHealthyResetAfter = 60 * time.Second
)

// GrpcCommunicator implements WorkflowCommunicator using gRPC
type GrpcCommunicator struct {
	serverAddress string
	conn          *grpc.ClientConn
	client        workflowsgrpc.EventServiceClient
	stream        grpc.BidiStreamingClient[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]
	serverName    string
	apiToken      string
	orgID         string // optional: sent as x-org-id metadata to pin the org
	useTLS        bool   // Determined at creation time
	// insecureSkipVerify, when true, disables TLS certificate verification.
	// Only takes effect when useTLS is true. Insecure: use only for self-signed
	// or otherwise untrusted certs in controlled environments.
	insecureSkipVerify bool

	// Channel for incoming events
	incomingEvents chan *types.EventMessage

	// Context for cancellation (process lifetime)
	ctx    context.Context
	cancel context.CancelFunc

	// Synchronization
	mu            sync.RWMutex
	connected     bool
	connCancel    context.CancelFunc // cancels the current connection's context
	reconnectCh   chan error         // signals connection loss with its cause (buffered 1, non-blocking sends)
	connectedCh   chan struct{}      // Closed on first successful connection
	connectedOnce sync.Once
	wg            sync.WaitGroup // tracks run() and per-connection goroutines

	// Reconnect callback - called when connection is re-established after a disconnect
	onReconnect func()

	// Configurable intervals (seconds). If 0, defaults are used by caller.
	reconnectIntervalSec   int
	healthcheckIntervalSec int
	pingIntervalSec        int // 0 = disabled

	// Backoff tuning (overridable in tests)
	maxBackoff        time.Duration
	healthyResetAfter time.Duration

	// Extra dial options, used by tests to dial in-memory listeners.
	dialOpts []grpc.DialOption
}

// NewGrpcCommunicator creates a new gRPC-based communicator
func NewGrpcCommunicator(serverAddress, serverName, apiToken string) *GrpcCommunicator {
	ctx, cancel := context.WithCancel(context.Background())

	return &GrpcCommunicator{
		serverAddress:          serverAddress,
		serverName:             serverName,
		apiToken:               apiToken,
		incomingEvents:         make(chan *types.EventMessage, 100), // Buffered channel
		ctx:                    ctx,
		cancel:                 cancel,
		reconnectCh:            make(chan error, 1),
		connectedCh:            make(chan struct{}),
		reconnectIntervalSec:   5,
		healthcheckIntervalSec: 30,
		maxBackoff:             defaultMaxBackoff,
		healthyResetAfter:      defaultHealthyResetAfter,
	}
}

// SetOrgID sets an optional organization id, sent as x-org-id gRPC metadata so
// the platform scopes registration to that org (for multi-org token owners).
func (gc *GrpcCommunicator) SetOrgID(orgID string) {
	gc.orgID = orgID
}

// SetInsecureSkipVerify controls whether TLS certificate verification is
// skipped. It only has an effect when the connection uses TLS. Skipping
// verification is insecure and should be reserved for self-signed or otherwise
// untrusted certificates in controlled environments.
func (gc *GrpcCommunicator) SetInsecureSkipVerify(skip bool) {
	gc.insecureSkipVerify = skip
}

// ShouldUseTLS determines if TLS should be used based on the server address
func ShouldUseTLS(address string) bool {
	// localhost, 127.0.0.1, [::1] = no TLS (development)
	if strings.HasPrefix(address, "localhost:") ||
		strings.HasPrefix(address, "127.0.0.1:") ||
		strings.HasPrefix(address, "[::1]:") {
		return false
	}
	// Everything else = TLS (production)
	return true
}

// NewGrpcCommunicatorWithOptions allows configuring buffer size and intervals.
func NewGrpcCommunicatorWithOptions(serverAddress, serverName, apiToken string, incomingBuffer, reconnectIntervalSec, healthcheckIntervalSec, pingIntervalSec int) *GrpcCommunicator {
	useTLS := ShouldUseTLS(serverAddress)
	return NewGrpcCommunicatorWithTLS(serverAddress, serverName, apiToken, incomingBuffer, reconnectIntervalSec, healthcheckIntervalSec, pingIntervalSec, useTLS)
}

// NewGrpcCommunicatorWithTLS allows explicit TLS control
func NewGrpcCommunicatorWithTLS(serverAddress, serverName, apiToken string, incomingBuffer, reconnectIntervalSec, healthcheckIntervalSec, pingIntervalSec int, useTLS bool) *GrpcCommunicator {
	if incomingBuffer <= 0 {
		incomingBuffer = 100
	}
	if reconnectIntervalSec <= 0 {
		reconnectIntervalSec = 5
	}
	if healthcheckIntervalSec <= 0 {
		healthcheckIntervalSec = 30
	}
	if pingIntervalSec < 0 {
		pingIntervalSec = 0
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &GrpcCommunicator{
		serverAddress:          serverAddress,
		serverName:             serverName,
		apiToken:               apiToken,
		useTLS:                 useTLS,
		incomingEvents:         make(chan *types.EventMessage, incomingBuffer),
		ctx:                    ctx,
		cancel:                 cancel,
		reconnectCh:            make(chan error, 1),
		connectedCh:            make(chan struct{}),
		reconnectIntervalSec:   reconnectIntervalSec,
		healthcheckIntervalSec: healthcheckIntervalSec,
		pingIntervalSec:        pingIntervalSec,
		maxBackoff:             defaultMaxBackoff,
		healthyResetAfter:      defaultHealthyResetAfter,
	}
}

// Connect starts the connection supervisor in the background. Exactly one
// supervisor goroutine exists for the life of the communicator; it owns all
// connect/reconnect decisions.
func (gc *GrpcCommunicator) Connect() error {
	gc.wg.Add(1)
	go gc.run()

	log.Printf("Started gRPC connection attempts to %s (retrying with backoff, %ds initial interval)", gc.serverAddress, gc.reconnectIntervalSec)
	return nil
}

// run is the connection supervisor: connect, watch the connection until it
// dies, tear it down, wait with backoff, repeat. It is the ONLY goroutine that
// dials or tears down connections, which guarantees a single live connection
// and no goroutine accumulation across reconnects.
func (gc *GrpcCommunicator) run() {
	defer gc.wg.Done()
	defer gc.disconnect()

	initial := time.Duration(gc.reconnectIntervalSec) * time.Second
	backoff := initial

	for {
		if gc.ctx.Err() != nil {
			return
		}

		if gc.attemptConnection() {
			connectedAt := time.Now()
			cause := gc.superviseConnection()
			gc.disconnect()
			gc.drainReconnectSignal() // discard stale signals from the connection just torn down

			if gc.ctx.Err() != nil {
				return
			}

			// A connection that stayed healthy for a while earns a backoff
			// reset. A connection that died immediately (e.g. rejected token)
			// must NOT reset it, or a connect-then-die loop would spin.
			if time.Since(connectedAt) >= gc.healthyResetAfter {
				backoff = initial
			}

			if isNonRetryable(cause) {
				// Cannot succeed by retrying (invalid/expired token, revoked
				// access). Go straight to the slowest cadence and say why.
				backoff = gc.maxBackoff
				if st, ok := status.FromError(cause); ok && st.Code() == codes.Unauthenticated {
					log.Printf("❌ Authentication failed: Invalid or expired API token. Retrying in %v. Error: %v", backoff, st.Message())
				} else {
					log.Printf("❌ Non-retryable error on stream: %v. Retrying in %v", cause, backoff)
				}
			} else {
				log.Printf("Connection lost (%v), reconnecting in %v", cause, backoff)
			}
		}

		// Wait before the next attempt — on EVERY path, including
		// connected-then-died. Jitter spreads simultaneous reconnects across
		// the fleet (e.g. after a server restart) to avoid thundering herds.
		select {
		case <-gc.ctx.Done():
			return
		case <-time.After(jitter(backoff)):
		}

		backoff *= 2
		if backoff > gc.maxBackoff {
			backoff = gc.maxBackoff
		}
	}
}

// superviseConnection blocks while the current connection is alive. It returns
// the cause of the connection's death: a stream/send error, or a synthetic
// error from the health checker. Returns gc.ctx.Err() on shutdown.
func (gc *GrpcCommunicator) superviseConnection() error {
	ticker := time.NewTicker(time.Duration(gc.healthcheckIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-gc.ctx.Done():
			return gc.ctx.Err()
		case err := <-gc.reconnectCh:
			return err
		case <-ticker.C:
			if err := gc.checkConnectionHealth(); err != nil {
				return err
			}
		}
	}
}

// drainReconnectSignal discards a pending reconnect signal, if any. Called
// after disconnect() and before the next attempt, so a late signal from the
// old connection cannot be mistaken for one from the new connection.
func (gc *GrpcCommunicator) drainReconnectSignal() {
	select {
	case <-gc.reconnectCh:
	default:
	}
}

// signalDisconnect notifies the supervisor that the connection died, with the
// cause. Non-blocking: if a signal is already pending, the first cause wins.
func (gc *GrpcCommunicator) signalDisconnect(cause error) {
	select {
	case gc.reconnectCh <- cause:
	default:
	}
}

// isNonRetryable reports whether an error can never be fixed by reconnecting
// (authentication/authorization failures). These get the slowest retry cadence.
func isNonRetryable(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	switch st.Code() {
	case codes.Unauthenticated, codes.PermissionDenied:
		return true
	}
	return false
}

// jitter returns a random duration in [d/2, d] to spread reconnects across a
// fleet of workers.
func jitter(d time.Duration) time.Duration {
	if d <= 1 {
		return d
	}
	half := d / 2
	return half + rand.N(half)
}

// attemptConnection tries to establish a single connection
func (gc *GrpcCommunicator) attemptConnection() bool {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	if gc.connected {
		return true
	}

	log.Printf("Attempting to connect to gRPC server at %s...", gc.serverAddress)

	// Warn if no API token is provided
	if gc.apiToken == "" {
		log.Printf("⚠️  Warning: No API token provided. Set SERVER_API_TOKEN environment variable for authentication.")
	}

	// Create gRPC connection with appropriate credentials
	var opts []grpc.DialOption

	if gc.useTLS {
		// Production: Use TLS with system certificates
		tlsConfig := &tls.Config{InsecureSkipVerify: gc.insecureSkipVerify}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
		if gc.insecureSkipVerify {
			log.Printf("Connecting with TLS to %s (certificate verification DISABLED)", gc.serverAddress)
		} else {
			log.Printf("Connecting with TLS to %s", gc.serverAddress)
		}
	} else {
		// Development: No TLS
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		log.Printf("Connecting without TLS to %s", gc.serverAddress)
	}

	// Disable gRPC's DNS-based service-config discovery. We don't use it, and it
	// makes the dns resolver issue an extra `TXT _grpc_config.<host>` lookup that
	// can stall for ~20s on networks whose resolver doesn't answer that record
	// (e.g. split-DNS / Tailscale setups where Go sends the query to a LAN
	// nameserver that drops it). The A record still resolves normally.
	opts = append(opts, grpc.WithDisableServiceConfig())
	opts = append(opts, gc.dialOpts...)

	conn, err := grpc.NewClient(gc.serverAddress, opts...)
	if err != nil {
		log.Printf("Failed to create gRPC client: %v", err)
		return false
	}

	gc.client = workflowsgrpc.NewEventServiceClient(conn)

	// Per-connection context: cancelled by disconnect(), which deterministically
	// ends this connection's stream, receive goroutine and ping loop.
	connCtx, connCancel := context.WithCancel(gc.ctx)

	// Attach authentication metadata
	streamCtx := connCtx
	if gc.apiToken != "" || gc.orgID != "" {
		pairs := map[string]string{}
		if gc.apiToken != "" {
			pairs["authorization"] = "Bearer " + gc.apiToken
		}
		if gc.orgID != "" {
			pairs["x-org-id"] = gc.orgID
		}
		streamCtx = metadata.NewOutgoingContext(streamCtx, metadata.New(pairs))
	}

	// Create bidirectional stream with authentication metadata
	stream, err := gc.client.Events(streamCtx)
	if err != nil {
		connCancel()
		conn.Close()
		// Check if this is an authentication error
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unauthenticated {
			if gc.apiToken == "" {
				log.Printf("❌ Authentication failed: No API token provided. Set SERVER_API_TOKEN environment variable.")
			} else {
				log.Printf("❌ Authentication failed: Invalid or expired API token. Error: %v", st.Message())
			}
		} else {
			log.Printf("❌ Failed to create event stream: %v", err)
		}
		return false
	}

	// Send initial registration message
	registrationMsg := &workflowsgrpc.GrpcEventMessage{
		Server:        gc.serverName,
		Event:         types.EventClientRegistration,
		Text:          "Client registration",
		CorrelationId: "",
	}

	if err := stream.Send(registrationMsg); err != nil {
		connCancel()
		conn.Close()
		log.Printf("Failed to register with server: %v", err)
		return false
	}

	// Success - store connection details
	gc.conn = conn
	gc.stream = stream
	gc.connCancel = connCancel
	gc.connected = true

	// Signal first connection (for WaitForConnection)
	// Also track if this is a reconnect for the callback
	isReconnect := true
	gc.connectedOnce.Do(func() {
		close(gc.connectedCh)
		isReconnect = false
	})

	// Start per-connection goroutines, bound to this connection's context and
	// stream so they exit when the connection is torn down.
	gc.wg.Add(1)
	go gc.receiveMessages(connCtx, stream)

	if gc.pingIntervalSec > 0 {
		gc.wg.Add(1)
		go gc.pingLoop(connCtx)
	}

	log.Printf("✅ gRPC client successfully connected to workflow server at %s", gc.serverAddress)

	// Call reconnect callback if this is a reconnection (not first connect)
	if isReconnect && gc.onReconnect != nil {
		go gc.onReconnect()
	}

	return true
}

// SendEvent sends an event to the workflow server via gRPC
func (gc *GrpcCommunicator) SendEvent(event *types.EventMessage) error {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	if !gc.connected || gc.stream == nil {
		return fmt.Errorf("not connected to workflow server")
	}

	// Convert to gRPC format
	grpcMsg, err := workflowsgrpc.ConvertToGRPC(event)
	if err != nil {
		return fmt.Errorf("failed to convert event to gRPC format: %w", err)
	}

	// Send the message
	if err := gc.stream.Send(grpcMsg); err != nil {
		log.Printf("Failed to send event via gRPC: %v", err)
		// Trigger reconnection
		gc.signalDisconnect(fmt.Errorf("send failed: %w", err))
		return fmt.Errorf("failed to send event: %w", err)
	}

	// Skip logging for ping keep-alive messages
	if event.Event != types.EventPing {
		log.Printf("Sent event via gRPC: %s", event.Event)
	}
	return nil
}

// ReceiveEvents returns a channel for receiving events from gRPC
func (gc *GrpcCommunicator) ReceiveEvents() <-chan *types.EventMessage {
	return gc.incomingEvents
}

// receiveMessages handles incoming messages from the given stream. It is bound
// to a single connection: it exits when the stream errors or connCtx is
// cancelled, signalling the supervisor with the cause.
func (gc *GrpcCommunicator) receiveMessages(connCtx context.Context, stream grpc.BidiStreamingClient[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]) {
	defer gc.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in receiveMessages: %v", r)
		}
	}()

	for {
		msg, err := stream.Recv()
		if err != nil {
			// Deliberate teardown (disconnect() or Close()) — exit silently.
			if connCtx.Err() != nil {
				return
			}
			if err == io.EOF {
				log.Println("gRPC stream closed by server")
			} else {
				log.Printf("Error receiving gRPC message: %v", err)
			}
			gc.signalDisconnect(err)
			return
		}

		// Convert from gRPC format
		eventMsg, err := workflowsgrpc.ConvertFromGRPC(msg)
		if err != nil {
			log.Printf("Failed to convert gRPC message: %v", err)
			continue
		}

		// Send to incoming events channel (non-blocking)
		select {
		case gc.incomingEvents <- eventMsg:
			// Skip logging for pong keep-alive messages
			if eventMsg.Event != types.EventPong {
				log.Printf("Received event via gRPC: %s", eventMsg.Event)
			}
		default:
			log.Printf("Incoming events channel full, dropping message: %s", eventMsg.Event)
		}
	}
}

// checkConnectionHealth returns an error if the connection is unhealthy.
func (gc *GrpcCommunicator) checkConnectionHealth() error {
	gc.mu.RLock()
	conn := gc.conn
	connected := gc.connected
	gc.mu.RUnlock()

	if !connected || conn == nil {
		return nil
	}

	state := conn.GetState()
	if state == connectivity.TransientFailure || state == connectivity.Shutdown {
		return fmt.Errorf("connection unhealthy (state: %v)", state)
	}
	return nil
}

// disconnect closes the current connection and cancels its context, which
// stops the connection's receive goroutine and ping loop.
func (gc *GrpcCommunicator) disconnect() {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	gc.connected = false

	if gc.connCancel != nil {
		gc.connCancel()
		gc.connCancel = nil
	}

	if gc.stream != nil {
		gc.stream.CloseSend()
		gc.stream = nil
	}

	if gc.conn != nil {
		gc.conn.Close()
		gc.conn = nil
	}
}

// Close closes the gRPC communicator and cleans up resources
func (gc *GrpcCommunicator) Close() error {
	gc.cancel()
	gc.disconnect()
	// Wait for the supervisor and all connection goroutines to exit before
	// closing incomingEvents, so no goroutine can send on a closed channel.
	gc.wg.Wait()
	close(gc.incomingEvents)

	log.Println("gRPC client closed")
	return nil
}

// IsConnected returns whether the communicator is currently connected
func (gc *GrpcCommunicator) IsConnected() bool {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.connected
}

// WaitForConnection blocks until the connection is established or timeout/cancellation occurs
func (gc *GrpcCommunicator) WaitForConnection(timeout time.Duration) error {
	select {
	case <-gc.connectedCh:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("connection timeout after %v", timeout)
	case <-gc.ctx.Done():
		return gc.ctx.Err()
	}
}

// SetOnReconnect sets a callback that will be called when the connection is re-established
// after a disconnect. This is useful for re-registering with the server.
func (gc *GrpcCommunicator) SetOnReconnect(callback func()) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.onReconnect = callback
}

// pingLoop sends ping messages at the configured interval while the connection
// that started it is alive.
func (gc *GrpcCommunicator) pingLoop(connCtx context.Context) {
	defer gc.wg.Done()

	ticker := time.NewTicker(time.Duration(gc.pingIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-connCtx.Done():
			return
		case <-ticker.C:
			gc.sendPing()
		}
	}
}

// sendPing sends a ping message to the workflow server
func (gc *GrpcCommunicator) sendPing() {
	pingEvent := &types.EventMessage{
		Server:        gc.serverName,
		Event:         types.EventPing,
		Text:          "ping",
		CorrelationID: "",
	}

	if err := gc.SendEvent(pingEvent); err != nil {
		log.Printf("Failed to send ping: %v", err)
	}
}

// conversion helpers removed in favor of workflowsgrpc converters
