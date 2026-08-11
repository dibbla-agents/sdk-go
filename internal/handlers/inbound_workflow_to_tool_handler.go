package handlers

import (
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
			if msg.Event == types.EventFunctionRequest {
				// Fail the caller fast instead of letting it wait out its timeout.
				fs := state.NewEventState(msg.Server, msg.Function, msg.Version, msg.Node, msg.Workflow, msg.Run, gs.ServerName, msg.CorrelationID)
				sendErrorEvent(gs, fs, "Worker overloaded: dispatcher queue full, request dropped")
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
		types.EventJobTrigger:
		return true
	default:
		return false
	}
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
