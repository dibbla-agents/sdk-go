package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/dibbla-agents/sdk-go/internal/basefunction"
	"github.com/dibbla-agents/sdk-go/internal/state"
	"github.com/dibbla-agents/sdk-go/internal/types"
	"log"
)

func HandleIncomingBroadcast(gs *state.GlobalState) {
	// In gRPC-only setup, broadcast messages are handled via the main workflow stream
}

func HandleListFunctions(gs *state.GlobalState, fs *state.EventState) {
	functions := []basefunction.FunctionDefinition{}
	gs.Functions.Range(func(key string, value basefunction.FunctionInterface) bool {
		functions = append(functions, value.GetFunctionDefinition())
		return true
	})

	payload, err := json.Marshal(functions)
	if err != nil {
		SendErrorEvent(gs, fs, fmt.Sprintf("Error marshalling functions: %v", err))
		return
	}

	event := types.EventMessage{
		Function:      fs.Function,
		Version:       fs.Version,
		Node:          fs.Node,
		Workflow:      fs.Workflow,
		Run:           fs.Run,
		Server:        gs.ServerName,
		Event:         types.EventResponseListFunctions,
		Text:          "List of functions",
		Meta:          nil,
		Payload:       &payload,
		CorrelationID: fs.CorrelationID,
	}

	if err := gs.WorkflowComm.SendEvent(&event); err != nil {
		log.Printf("Failed to send list functions response: %v", err)
	}
}

// HandleListCapabilityProviders announces the worker's registered capability
// providers to the workflow server. A worker with no providers sends nothing,
// so the wire behavior of existing workers is unchanged.
func HandleListCapabilityProviders(gs *state.GlobalState, fs *state.EventState) {
	if len(gs.CapabilityProviders) == 0 {
		return
	}

	payload, err := json.Marshal(gs.CapabilityProviders)
	if err != nil {
		SendErrorEvent(gs, fs, fmt.Sprintf("Error marshalling capability providers: %v", err))
		return
	}

	event := types.EventMessage{
		Function:      fs.Function,
		Version:       fs.Version,
		Node:          fs.Node,
		Workflow:      fs.Workflow,
		Run:           fs.Run,
		Server:        gs.ServerName,
		Event:         types.EventResponseListCapabilityProviders,
		Text:          "List of capability providers",
		Meta:          nil,
		Payload:       &payload,
		CorrelationID: fs.CorrelationID,
	}

	if err := gs.WorkflowComm.SendEvent(&event); err != nil {
		log.Printf("Failed to send list capability providers response: %v", err)
	}
}

func handleServerName(gs *state.GlobalState, fs *state.EventState) {
	event := types.EventMessage{
		Function:      fs.Function,
		Version:       fs.Version,
		Node:          fs.Node,
		Workflow:      fs.Workflow,
		Run:           fs.Run,
		Server:        gs.ServerName,
		Event:         types.EventResponseServerName,
		Text:          gs.ServerName,
		Meta:          nil,
		Payload:       nil,
		CorrelationID: fs.CorrelationID,
	}

	if err := gs.WorkflowComm.SendEvent(&event); err != nil {
		log.Printf("Failed to send server name response: %v", err)
	}
}

// func handleFunctionDefinition(gs *state.GlobalState, fs *state.EventState) {
// 	function, ok := functions.gs, fs.Function, fs.Version)
// 	if !ok {
// 		SendErrorEvent(gs, fs, "Function not found")
// 		return
// 	}

// 	payload, err := json.Marshal(function.GetFunctionDefinition())
// 	if err != nil {
// 		SendErrorEvent(gs, fs, fmt.Sprintf("Error marshalling function definition: %v", err))
// 		return
// 	}

// 	SendEventWithPayload(
// 		gs,
// 		fs,
// 		gs.Redis.WorkflowOutStream,
// 		"response_function_definition",
// 		"Function definition",
// 		nil,
// 		&payload,
// 	)
// }
