package store_chat_history

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/basefunction"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/state"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
	"strings"
	"time"
)

const FUNCTION_NAME = "store_chat_history"
const FUNCTION_VERSION = "1.0.0"
const FUNCTION_DESCRIPTION = `
Appends the input text to a per-workflow chat history stored via gRPC store and returns the full history as a single string.
`

type Inputs struct {
	Text string `json:"text"`
}

type Outputs struct {
	History string `json:"history"`
}

// historyKey is the key used within the workflow's store namespace
const historyKey = "chat_history"

func handler(inputs Inputs, event *types.EventMessage, gs *state.GlobalState) (Outputs, error) {
	if gs == nil || gs.GrpcStore == nil {
		return Outputs{}, fmt.Errorf("grpc store client not available")
	}

	// Determine workflow scope; fall back to server-wide if no workflow present
	workflowID := event.Workflow
	if workflowID == "" {
		workflowID = "__global__"
	}

	// Fetch existing history (JSON array of strings). Treat any error as empty history for simplicity.
	var history []string
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if data, err := gs.GrpcStore.Get(ctx, workflowID, historyKey); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &history)
	}

	// Append new message
	history = append(history, inputs.Text)

	// Persist back to store
	bytes, err := json.Marshal(history)
	if err != nil {
		return Outputs{}, fmt.Errorf("failed to marshal history: %w", err)
	}

	if err := gs.GrpcStore.Set(context.Background(), workflowID, historyKey, bytes); err != nil {
		return Outputs{}, fmt.Errorf("failed to persist history: %w", err)
	}

	// Return joined history for simplicity
	return Outputs{History: strings.Join(history, "\n")}, nil
}

// NewFunction builds the function instance for registration
func NewFunction(gs *state.GlobalState) basefunction.FunctionInterface {
	return basefunction.NewFunction(
		FUNCTION_NAME,
		FUNCTION_VERSION,
		FUNCTION_DESCRIPTION,
		func(inputs Inputs, eventState *types.EventMessage) (Outputs, error) {
			return handler(inputs, eventState, gs)
		},
		[]string{"example", "store", "chat"},
	)
}

