package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sdk "github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/state"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
)

type StoreChatHistoryInputs struct {
	Text string `json:"text"`
}

type StoreChatHistoryOutputs struct {
	History string `json:"history"`
}

// StoreChatHistoryFunction appends input text to per-workflow history using the gRPC store
func StoreChatHistoryFunction() sdk.FunctionBuilder {
	return sdk.NewFunction[StoreChatHistoryInputs, StoreChatHistoryOutputs](
		"store_chat_history",
		"1.0.0",
		"Appends the input text to a per-workflow chat history stored via gRPC store and returns the full history.",
	).WithCacheTTL(0).WithHandler(func(inputs StoreChatHistoryInputs, event *types.EventMessage, gs *state.GlobalState) (StoreChatHistoryOutputs, error) {
		if gs == nil || gs.GrpcStore == nil {
			return StoreChatHistoryOutputs{}, fmt.Errorf("grpc store client not available")
		}

		workflowID := event.Workflow
		if workflowID == "" {
			workflowID = "__global__"
		}

		const historyKey = "chat_history"

		// Special case: if input is "clear", clear the stored value to an empty string and return empty output
		if inputs.Text == "clear" || inputs.Text == "clear_history" {
			if err := gs.GrpcStore.Set(context.Background(), workflowID, historyKey, []byte("")); err != nil {
				return StoreChatHistoryOutputs{}, fmt.Errorf("failed to clear history: %w", err)
			}
			return StoreChatHistoryOutputs{History: "cleared"}, nil
		}

		var history []string
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if data, err := gs.GrpcStore.Get(ctx, workflowID, historyKey); err == nil && len(data) > 0 {
			_ = json.Unmarshal(data, &history)
		}

		history = append(history, inputs.Text)
		bytes, err := json.Marshal(history)
		if err != nil {
			return StoreChatHistoryOutputs{}, fmt.Errorf("failed to marshal history: %w", err)
		}
		if err := gs.GrpcStore.Set(context.Background(), workflowID, historyKey, bytes); err != nil {
			return StoreChatHistoryOutputs{}, fmt.Errorf("failed to persist history: %w", err)
		}
		return StoreChatHistoryOutputs{History: strings.Join(history, "\n")}, nil
	}).WithTags("example", "store", "chat")
}
