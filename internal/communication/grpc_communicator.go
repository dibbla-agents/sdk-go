package communication

import (
	"context"
	"fmt"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/workflowsgrpc"
	"io"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GrpcCommunicator implements WorkflowCommunicator using gRPC
type GrpcCommunicator struct {
	serverAddress string
	conn          *grpc.ClientConn
	client        workflowsgrpc.EventServiceClient
	stream        grpc.BidiStreamingClient[workflowsgrpc.GrpcEventMessage, workflowsgrpc.GrpcEventMessage]
	serverName    string
	apiToken      string

	// Channel for incoming events
	incomingEvents chan *types.EventMessage

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Synchronization
	mu          sync.RWMutex
	connected   bool
	reconnectCh chan struct{}

	// Configurable intervals (seconds). If 0, defaults are used by caller.
	reconnectIntervalSec   int
	healthcheckIntervalSec int
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
		reconnectCh:            make(chan struct{}, 1),
		reconnectIntervalSec:   5,
		healthcheckIntervalSec: 30,
	}
}

// NewGrpcCommunicatorWithOptions allows configuring buffer size and intervals.
func NewGrpcCommunicatorWithOptions(serverAddress, serverName, apiToken string, incomingBuffer, reconnectIntervalSec, healthcheckIntervalSec int) *GrpcCommunicator {
	if incomingBuffer <= 0 {
		incomingBuffer = 100
	}
	if reconnectIntervalSec <= 0 {
		reconnectIntervalSec = 5
	}
	if healthcheckIntervalSec <= 0 {
		healthcheckIntervalSec = 30
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &GrpcCommunicator{
		serverAddress:          serverAddress,
		serverName:             serverName,
		apiToken:               apiToken,
		incomingEvents:         make(chan *types.EventMessage, incomingBuffer),
		ctx:                    ctx,
		cancel:                 cancel,
		reconnectCh:            make(chan struct{}, 1),
		reconnectIntervalSec:   reconnectIntervalSec,
		healthcheckIntervalSec: healthcheckIntervalSec,
	}
}

// Connect starts the connection process and retries until successful
func (gc *GrpcCommunicator) Connect() error {
	// Start connection attempts in background
	go gc.connectionLoop()

	log.Printf("Started gRPC connection attempts to %s (will retry every 5 seconds until successful)", gc.serverAddress)
	return nil
}

// connectionLoop continuously attempts to connect with 5-second intervals
func (gc *GrpcCommunicator) connectionLoop() {
	for {
		select {
		case <-gc.ctx.Done():
			return
		default:
		}

		if gc.attemptConnection() {
			// Successfully connected, start monitoring
			go gc.connectionMonitor()
			return
		}

		// Wait before next attempt
		select {
		case <-gc.ctx.Done():
			return
		case <-time.After(time.Duration(gc.reconnectIntervalSec) * time.Second):
			continue
		}
	}
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

	// Create gRPC connection without blocking
	conn, err := grpc.NewClient(
		gc.serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("Failed to create gRPC client: %v", err)
		return false
	}

	// Test connection with a short timeout
	ctx, cancel := context.WithTimeout(gc.ctx, 5*time.Second)
	defer cancel()

	// Wait for connection to be ready or idle (idle is acceptable since RPCs will trigger connection)
	state := conn.GetState()
	for state != connectivity.Ready && state != connectivity.Idle {
		log.Printf("connection state: %v", state)
		if !conn.WaitForStateChange(ctx, state) {
			conn.Close()
			log.Printf("Connection timeout to %s", gc.serverAddress)
			return false
		}

		// Check if context was cancelled
		select {
		case <-ctx.Done():
			conn.Close()
			log.Printf("Connection attempt cancelled")
			return false
		default:
		}

		state = conn.GetState()
	}

	gc.client = workflowsgrpc.NewEventServiceClient(conn)

	// Create context with authentication metadata
	streamCtx := gc.ctx
	if gc.apiToken != "" {
		md := metadata.New(map[string]string{
			"authorization": "Bearer " + gc.apiToken,
		})
		streamCtx = metadata.NewOutgoingContext(streamCtx, md)
	}

	// Create bidirectional stream with authentication metadata
	stream, err := gc.client.Events(streamCtx)
	if err != nil {
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
		conn.Close()
		log.Printf("Failed to register with server: %v", err)
		return false
	}

	// Success - store connection details
	gc.conn = conn
	gc.stream = stream
	gc.connected = true

	// Start message handler
	go gc.receiveMessages()

	log.Printf("✅ gRPC client successfully connected to workflow server at %s", gc.serverAddress)
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
		select {
		case gc.reconnectCh <- struct{}{}:
		default:
		}
		return fmt.Errorf("failed to send event: %w", err)
	}

	log.Printf("Sent event via gRPC: %s", event.Event)
	return nil
}

// ReceiveEvents returns a channel for receiving events from gRPC
func (gc *GrpcCommunicator) ReceiveEvents() <-chan *types.EventMessage {
	return gc.incomingEvents
}

// receiveMessages handles incoming messages from the server
func (gc *GrpcCommunicator) receiveMessages() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in receiveMessages: %v", r)
		}
	}()

	for {
		select {
		case <-gc.ctx.Done():
			return
		default:
		}

		gc.mu.RLock()
		stream := gc.stream
		connected := gc.connected
		gc.mu.RUnlock()

		if !connected || stream == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		msg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Println("gRPC stream closed by server")
			} else {
				log.Printf("Error receiving gRPC message: %v", err)
			}

			// Trigger reconnection
			select {
			case gc.reconnectCh <- struct{}{}:
			default:
			}
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
			log.Printf("Received event via gRPC: %s", eventMsg.Event)
		default:
			log.Printf("Incoming events channel full, dropping message: %s", eventMsg.Event)
		}
	}
}

// connectionMonitor monitors connection health and handles reconnection
func (gc *GrpcCommunicator) connectionMonitor() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in connectionMonitor: %v", r)
		}
	}()

	ticker := time.NewTicker(time.Duration(gc.healthcheckIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-gc.ctx.Done():
			return
		case <-gc.reconnectCh:
			gc.handleReconnection()
		case <-ticker.C:
			gc.checkConnectionHealth()
		}
	}
}

// checkConnectionHealth checks if the connection is still healthy
func (gc *GrpcCommunicator) checkConnectionHealth() {
	gc.mu.RLock()
	conn := gc.conn
	connected := gc.connected
	gc.mu.RUnlock()

	if !connected || conn == nil {
		return
	}

	state := conn.GetState()
	if state == connectivity.TransientFailure || state == connectivity.Shutdown {
		log.Printf("Connection unhealthy (state: %v), triggering reconnection", state)
		select {
		case gc.reconnectCh <- struct{}{}:
		default:
		}
	}
}

// handleReconnection handles reconnection logic
func (gc *GrpcCommunicator) handleReconnection() {
	log.Println("Connection lost, attempting to reconnect...")

	gc.disconnect()

	// Start the connection loop again (it will retry every 5 seconds)
	go gc.connectionLoop()
}

// disconnect closes the current connection
func (gc *GrpcCommunicator) disconnect() {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	gc.connected = false

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

// conversion helpers removed in favor of workflowsgrpc converters
