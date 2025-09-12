package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/communication"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/correlation"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/models"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/utils"
	"log"
	"time"
)

type RpcClient struct {
	communicator communication.WorkflowCommunicator
	router       *correlation.Router
}

func NewRpcClient(workflowOut chan types.EventMessage) *RpcClient {
	return &RpcClient{
		communicator: nil, // Legacy constructor - communicator will be nil
		router:       correlation.NewRouter(),
	}
}

// NewRpcClientWithCommunicator creates a new RPC client with the communication layer
func NewRpcClientWithCommunicator(communicator communication.WorkflowCommunicator) *RpcClient {
	return &RpcClient{
		communicator: communicator,
		router:       correlation.NewRouter(),
	}
}

// SendStatusEvent sends a status update event with the given text and optional payload
func (r *RpcClient) SendStatusEvent(eventState *types.EventMessage, text string, payload interface{}) error {
	if eventState == nil {
		return fmt.Errorf("eventState is nil")
	}
	var payloadBytes *[]byte

	if payload != nil {
		bytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal status payload: %w", err)
		}
		payloadBytes = &bytes
	}

	statusMsg := types.EventMessage{
		Function:      eventState.Function,
		Version:       eventState.Version,
		Node:          eventState.Node,
		Workflow:      eventState.Workflow,
		Run:           eventState.Run,
		Server:        eventState.Server,
		Event:         types.EventStatusMessage,
		Text:          text,
		Meta:          nil,
		Payload:       payloadBytes,
		CorrelationID: eventState.CorrelationID,
	}

	if r.communicator != nil {
		return r.communicator.SendEvent(&statusMsg)
	}

	// Legacy fallback - this should not happen in new code
	return fmt.Errorf("no communication method available")
}

func (r *RpcClient) Call(timeoutMinutes int, executionNode *models.Node, eventState *types.EventMessage, payload interface{}) ([]byte, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMinutes)*time.Minute)
	defer cancel()

	correlationID := utils.UID()
	responseChan := r.router.Register(correlationID, 1)
	defer r.router.Remove(correlationID)

	// Create and send the event message
	var eventMsg types.EventMessage
	if executionNode.Type != "flow_tool" {
		eventMsg = types.EventMessage{
			Function:      executionNode.Data.Function.Name,
			Version:       executionNode.Data.Function.Version,
			Server:        executionNode.Data.Function.Server,
			Node:          executionNode.ID,
			Workflow:      eventState.Workflow,
			Run:           eventState.Run,
			Event:         types.EventFunctionRequest,
			Text:          "Node " + executionNode.ID + " is invoking a function from a tool server",
			Meta:          &map[string]interface{}{"calling_server": eventState.Server},
			Payload:       &payloadBytes,
			CorrelationID: correlationID,
		}
		// r.workflowOut <- eventMsg
	} else {
		eventMsg = types.EventMessage{
			Server:        eventState.Server,
			Node:          executionNode.ID,
			Workflow:      eventState.Workflow,
			Run:           eventState.Run,
			Event:         types.EventFlowNodeRequest,
			Text:          "Node " + executionNode.ID + " is invoking a flow from a tool server",
			Meta:          nil,
			Payload:       &payloadBytes,
			CorrelationID: correlationID,
		}
	}
	if r.communicator != nil {
		if err := r.communicator.SendEvent(&eventMsg); err != nil {
			return nil, fmt.Errorf("failed to send event: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no communication method available")
	}

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		log.Println("RpcClient: Received function response for correlation ID: " + correlationID)
		if response.Payload == nil {
			return nil, fmt.Errorf("received empty payload")
		}
		return *response.Payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ... rest of the code remains the same ...
func (r *RpcClient) HandleCallResponse(response types.EventMessage) {
	if response.Event != types.EventFunctionResponse {
		return
	}

	r.router.Deliver(response.CorrelationID, response)
}
