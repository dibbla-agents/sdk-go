package handlers

import (
	"fmt"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/state"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
	"log"
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

	gs.Dispatcher.Register(types.EventFunctionResponse, func(message *types.EventMessage) {
		gs.RpcClient.HandleCallResponse(*message)
	})

	gs.Dispatcher.Register(types.EventCacheGetResponse, func(message *types.EventMessage) {
		if gs.GrpcCache != nil {
			gs.GrpcCache.HandleResponse(*message)
		}
	})
	gs.Dispatcher.Register(types.EventCacheSetResponse, func(message *types.EventMessage) {
		if gs.GrpcCache != nil {
			gs.GrpcCache.HandleResponse(*message)
		}
	})

	gs.Dispatcher.Register(types.EventStoreGetResponse, func(message *types.EventMessage) {
		if gs.GrpcStore != nil {
			gs.GrpcStore.HandleResponse(*message)
		}
	})
	gs.Dispatcher.Register(types.EventStoreSetResponse, func(message *types.EventMessage) {
		if gs.GrpcStore != nil {
			gs.GrpcStore.HandleResponse(*message)
		}
	})

	gs.Dispatcher.Register(types.EventRequestListFunctions, func(message *types.EventMessage) {
		fs := state.NewEventState(message.Server, message.Function, message.Version, message.Node, message.Workflow, message.Run, gs.ServerName, message.CorrelationID)
		HandleListFunctions(gs, fs)
	})
	gs.Dispatcher.Register(types.EventRequestServerName, func(message *types.EventMessage) {
		fs := state.NewEventState(message.Server, message.Function, message.Version, message.Node, message.Workflow, message.Run, gs.ServerName, message.CorrelationID)
		handleServerName(gs, fs)
	})
	gs.Dispatcher.Register(types.EventRequestServerInfo, func(message *types.EventMessage) {
		fs := state.NewEventState(message.Server, message.Function, message.Version, message.Node, message.Workflow, message.Run, gs.ServerName, message.CorrelationID)
		handleServerName(gs, fs)
		HandleListFunctions(gs, fs)
	})

	// Read from communicator and dispatch
	incomingEvents := gs.WorkflowComm.ReceiveEvents()
	for msg := range incomingEvents {
		log.Println("Received workflow message with event: " + msg.Event + " and workflow: " + msg.Workflow)
		if msg.Workflow == "" && msg.Event != types.EventCacheGetResponse && msg.Event != types.EventCacheSetResponse && msg.Event != types.EventStoreGetResponse && msg.Event != types.EventStoreSetResponse && msg.Event != types.EventRequestServerInfo && msg.Event != types.EventRequestServerName && msg.Event != types.EventRequestListFunctions {
			log.Println("Workflow is empty, skipping")
			continue
		}
		gs.Dispatcher.Dispatch(msg)
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
