package handlers

import (
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/state"
)

func Activate(gs *state.GlobalState) {
	go HandleIncomingBroadcast(gs)
	go HandleIncomingWorkflow(gs)
}

func registerServer(gs *state.GlobalState) {
	fs := state.EventState{
		Function: "",
		Version:  "",
		Node:     "",
		Workflow: "",
	}
	handleServerName(gs, &fs)
	HandleListFunctions(gs, &fs)
}

// Redis-based publishing removed in gRPC-only setup
