package examples

import (
	"context"
	"log"
	"time"

	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/workflowsgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ExampleClient demonstrates how to connect to and use the gRPC server
func ExampleClient(serverAddr string) error {
	// Set up a connection to the server
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	// Create a client
	client := workflowsgrpc.NewEventServiceClient(conn)

	// Create a bidirectional stream
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.Events(ctx)
	if err != nil {
		return err
	}

	// Send a test message
	testMessage := &workflowsgrpc.GrpcEventMessage{
		Function:      "test_function",
		Node:          "test_node",
		Workflow:      "test_workflow",
		Version:       "1.0.0",
		Server:        "test_server",
		Event:         "test_event",
		Text:          "Hello from gRPC client!",
		Run:           "test_run",
		CorrelationId: "test_correlation_id",
	}

	if err := stream.Send(testMessage); err != nil {
		return err
	}

	log.Printf("Sent message: %s", testMessage.Event)

	// Close the send direction
	if err := stream.CloseSend(); err != nil {
		return err
	}

	// Wait for the stream to finish
	for {
		_, err := stream.Recv()
		if err != nil {
			break // Stream is done
		}
	}

	log.Println("Client finished successfully")
	return nil
}
