package handlers

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/dibbla-agents/sdk-go/internal/state"
	"github.com/dibbla-agents/sdk-go/internal/types"
)

func HandleIncomingWorkflow(gs *state.GlobalState) {
	// Register handlers on dispatcher
	gs.Dispatcher.Register(types.EventFunctionRequest, func(message *types.EventMessage) {
		fs := state.NewEventState(message.Server, message.Function, message.Version, message.Node, message.Workflow, message.Run, gs.ServerName, message.CorrelationID)
		log.Println("Received function request: " + message.Function)
		functionKey := getFunctionKey(fs)
		function, ok := gs.Functions.Load(functionKey)
		if !ok {
			sendErrorEvent(gs, fs, "Function not found")
			return
		}
		outputs, err := function.Execute(message.Payload, message)
		if err != nil {
			sendErrorEvent(gs, fs, fmt.Sprintf("Function execution failed: %v", err))
			return
		}
		sendFunctionResponse(gs, fs, outputs)
	})

	gs.Dispatcher.RegisterDirect(types.EventFunctionResponse, func(message *types.EventMessage) {
		gs.RpcClient.HandleCallResponse(*message)
	})

	gs.Dispatcher.RegisterDirect(types.EventCacheGetResponse, func(message *types.EventMessage) {
		if gs.GrpcCache != nil {
			gs.GrpcCache.HandleResponse(*message)
		}
	})
	gs.Dispatcher.RegisterDirect(types.EventCacheSetResponse, func(message *types.EventMessage) {
		if gs.GrpcCache != nil {
			gs.GrpcCache.HandleResponse(*message)
		}
	})

	gs.Dispatcher.RegisterDirect(types.EventStoreGetResponse, func(message *types.EventMessage) {
		if gs.GrpcStore != nil {
			gs.GrpcStore.HandleResponse(*message)
		}
	})
	gs.Dispatcher.RegisterDirect(types.EventStoreSetResponse, func(message *types.EventMessage) {
		if gs.GrpcStore != nil {
			gs.GrpcStore.HandleResponse(*message)
		}
	})

	gs.Dispatcher.RegisterDirect(types.EventOAuthTokenResponse, func(message *types.EventMessage) {
		if gs.OAuth != nil {
			gs.OAuth.HandleResponse(*message)
		}
	})
	gs.Dispatcher.RegisterDirect(types.EventOAuthStatusResponse, func(message *types.EventMessage) {
		if gs.OAuth != nil {
			gs.OAuth.HandleResponse(*message)
		}
	})
	gs.Dispatcher.RegisterDirect(types.EventOAuthError, func(message *types.EventMessage) {
		if gs.OAuth != nil {
			gs.OAuth.HandleResponse(*message)
		}
	})

	gs.Dispatcher.RegisterDirect(types.EventRequestListFunctions, func(message *types.EventMessage) {
		fs := state.NewEventState(message.Server, message.Function, message.Version, message.Node, message.Workflow, message.Run, gs.ServerName, message.CorrelationID)
		HandleListFunctions(gs, fs)
	})
	gs.Dispatcher.RegisterDirect(types.EventRequestServerName, func(message *types.EventMessage) {
		fs := state.NewEventState(message.Server, message.Function, message.Version, message.Node, message.Workflow, message.Run, gs.ServerName, message.CorrelationID)
		handleServerName(gs, fs)
	})
	gs.Dispatcher.RegisterDirect(types.EventRequestServerInfo, func(message *types.EventMessage) {
		fs := state.NewEventState(message.Server, message.Function, message.Version, message.Node, message.Workflow, message.Run, gs.ServerName, message.CorrelationID)
		handleServerName(gs, fs)
		HandleListFunctions(gs, fs)
		HandleListCapabilityProviders(gs, fs)
	})

	// Provider invocation route (DIB-152): decode the request, look up the
	// registered seat handler by "<capability>/<name>", run it, and reply on
	// the same correlation ID. A request for a provider without a live
	// handler gets an explicit error back instead of a silent timeout.
	gs.Dispatcher.Register(types.EventCapabilityProviderRequest, func(message *types.EventMessage) {
		fs := state.NewEventState(message.Server, message.Function, message.Version, message.Node, message.Workflow, message.Run, gs.ServerName, message.CorrelationID)
		handleCapabilityProviderRequest(gs, fs, message)
	})

	// Catalog pre-sync (DIB-152): a one-way push of the full stub set before
	// the first query. This SDK version keeps providers stateless (per-query
	// stubs are always sent), so the catalog is accepted and ignored — logged
	// only. Future embedding providers can hook this to pre-index.
	gs.Dispatcher.Register(types.EventCapabilityCatalog, func(message *types.EventMessage) {
		log.Println("Received capability catalog pre-sync (ignored by this SDK version)")
	})

	// Read from communicator and dispatch
	incomingEvents := gs.WorkflowComm.ReceiveEvents()
	for msg := range incomingEvents {
		// Skip pong keep-alive responses entirely - they don't need processing
		if msg.Event == types.EventPong {
			continue
		}
		log.Println("Received workflow message with event: " + msg.Event + " and workflow: " + msg.Workflow)
		if msg.Workflow == "" && !isWorkflowOptionalEvent(msg.Event) {
			log.Println("Workflow is empty, skipping")
			continue
		}
		if !gs.Dispatcher.Dispatch(msg) {
			// Worker pool queue is full. Dropping here instead of blocking
			// keeps this loop draining incomingEvents, so responses and
			// control events (which dispatch direct) still get through and
			// parked handlers can complete and free the pool (FAT-19).
			log.Printf("Dispatcher queue full, dropping event: %s (workflow: %s)", msg.Event, msg.Workflow)
			switch msg.Event {
			case types.EventFunctionRequest:
				// Fail the caller fast instead of letting it wait out its timeout.
				fs := state.NewEventState(msg.Server, msg.Function, msg.Version, msg.Node, msg.Workflow, msg.Run, gs.ServerName, msg.CorrelationID)
				sendErrorEvent(gs, fs, "Worker overloaded: dispatcher queue full, request dropped")
			case types.EventCapabilityProviderRequest:
				// Same fast-fail for capability provider calls, on the seat's
				// structured response channel so the engine's correlation RPC
				// resolves with a coded failure instead of timing out.
				fs := state.NewEventState(msg.Server, msg.Function, msg.Version, msg.Node, msg.Workflow, msg.Run, gs.ServerName, msg.CorrelationID)
				sendCapabilityProviderErrorResponse(gs, fs, "worker overloaded: dispatcher queue full, capability provider request dropped")
			}
		}
	}
}

// isWorkflowOptionalEvent returns true for events that don't require a workflow field
func isWorkflowOptionalEvent(event string) bool {
	switch event {
	case types.EventCacheGetResponse,
		types.EventCacheSetResponse,
		types.EventStoreGetResponse,
		types.EventStoreSetResponse,
		types.EventOAuthTokenResponse,
		types.EventOAuthStatusResponse,
		types.EventOAuthError,
		types.EventRequestServerInfo,
		types.EventRequestServerName,
		types.EventRequestListFunctions,
		types.EventCapabilityProviderRequest,
		types.EventCapabilityCatalog,
		types.EventJobTrigger:
		return true
	default:
		return false
	}
}

// handleCapabilityProviderRequest runs a registered seat handler and replies
// with a capability_provider_response. When no handler is registered for the
// requested provider (or the payload names none), it sends a structured
// error response so the engine fails the node fail-fast with a clear cause
// rather than waiting out the call timeout.
func handleCapabilityProviderRequest(gs *state.GlobalState, fs *state.EventState, message *types.EventMessage) {
	var req types.CapabilityProviderRequest
	if message.Payload != nil {
		if err := json.Unmarshal(*message.Payload, &req); err != nil {
			sendCapabilityProviderErrorResponse(gs, fs, "capability provider request payload is not valid JSON: "+err.Error())
			return
		}
	}

	key := req.Capability + "/" + req.Provider
	handler, ok := gs.CapabilityProviderHandlers[key]
	if !ok || handler == nil {
		sendCapabilityProviderErrorResponse(gs, fs,
			fmt.Sprintf("capability provider %q for capability %q is not implemented by this server", req.Provider, req.Capability))
		return
	}

	respPayload, err := handler(message.Payload)
	if err != nil {
		sendCapabilityProviderErrorResponse(gs, fs, err.Error())
		return
	}
	sendCapabilityProviderResponse(gs, fs, respPayload)
}

func sendCapabilityProviderResponse(gs *state.GlobalState, fs *state.EventState, payload *[]byte) {
	event := types.EventMessage{
		Function:      fs.Function,
		Version:       fs.Version,
		Node:          fs.Node,
		Workflow:      fs.Workflow,
		Run:           fs.Run,
		Event:         types.EventCapabilityProviderResponse,
		Payload:       payload,
		CorrelationID: fs.CorrelationID,
	}
	if err := gs.WorkflowComm.SendEvent(&event); err != nil {
		log.Printf("Failed to send capability provider response: %v", err)
	}
}

// sendCapabilityProviderErrorResponse replies on the response channel with an
// error-bearing response payload (not an "error" event) so the engine's
// correlation RPC resolves promptly with a coded provider failure.
func sendCapabilityProviderErrorResponse(gs *state.GlobalState, fs *state.EventState, errText string) {
	payload, err := json.Marshal(types.CapabilityProviderResponse{Error: errText})
	if err != nil {
		log.Printf("Failed to marshal capability provider error response: %v", err)
		return
	}
	sendCapabilityProviderResponse(gs, fs, &payload)
}

func sendErrorEvent(gs *state.GlobalState, fs *state.EventState, errorText string) {
	event := types.EventMessage{
		Function:      fs.Function,
		Version:       fs.Version,
		Node:          fs.Node,
		Workflow:      fs.Workflow,
		Run:           fs.Run,
		Event:         types.EventError,
		Text:          errorText,
		Meta:          nil,
		Payload:       nil,
		CorrelationID: fs.CorrelationID,
	}

	if err := gs.WorkflowComm.SendEvent(&event); err != nil {
		log.Printf("Failed to send error event: %v", err)
	}
}

func sendFunctionResponse(gs *state.GlobalState, fs *state.EventState, payload *[]byte) {
	event := types.EventMessage{
		Function:      fs.Function,
		Version:       fs.Version,
		Node:          fs.Node,
		Workflow:      fs.Workflow,
		Run:           fs.Run,
		Event:         types.EventFunctionResponse,
		Text:          "",
		Meta:          nil,
		Payload:       payload,
		CorrelationID: fs.CorrelationID,
	}

	if err := gs.WorkflowComm.SendEvent(&event); err != nil {
		log.Printf("Failed to send function response: %v", err)
	}
}

func getFunctionKey(es *state.EventState) string {
	return types.FunctionKey(es.FunctionServer, es.Function, es.Version)
}
