package models

import "github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"

type EventState struct {
	Function      string `json:"function"`
	Node          string `json:"node"`
	Workflow      string `json:"workflow"`
	Version       string `json:"version"`
	Server        string `json:"server"`
	Run           string `json:"run"`
	CorrelationID string `json:"correlation_id"`
}

func NewEventState(msg *types.EventMessage) EventState {
	return EventState{
		Function:      msg.Function,
		Node:          msg.Node,
		Workflow:      msg.Workflow,
		Version:       msg.Version,
		Server:        msg.Server,
		Run:           msg.Run,
		CorrelationID: msg.CorrelationID,
	}
}
