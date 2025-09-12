package handlers

import (
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/state"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
	"log"
)

func SendErrorEvent(gs *state.GlobalState, fs *state.EventState, errorText string) {
	event := types.EventMessage{
		Server:        gs.ServerName,
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

func SendSimpleEvent(gs *state.GlobalState, fs *state.EventState, eventType string, eventText string) {
	event := types.EventMessage{
		Server:        gs.ServerName,
		Function:      fs.Function,
		Version:       fs.Version,
		Node:          fs.Node,
		Workflow:      fs.Workflow,
		Run:           fs.Run,
		Event:         eventType,
		Text:          eventText,
		Meta:          nil,
		Payload:       nil,
		CorrelationID: fs.CorrelationID,
	}

	if err := gs.WorkflowComm.SendEvent(&event); err != nil {
		log.Printf("Failed to send simple event: %v", err)
	}
}

func SendEventWithPayload(gs *state.GlobalState, fs *state.EventState, eventType string, eventText string, meta *map[string]any, payload *[]byte) {
	event := types.EventMessage{
		Server:        gs.ServerName,
		Function:      fs.Function,
		Version:       fs.Version,
		Node:          fs.Node,
		Workflow:      fs.Workflow,
		Run:           fs.Run,
		Event:         eventType,
		Text:          eventText,
		Meta:          meta,
		Payload:       payload,
		CorrelationID: fs.CorrelationID,
	}

	if err := gs.WorkflowComm.SendEvent(&event); err != nil {
		log.Printf("Failed to send event with payload: %v", err)
	}
}
