package input_function

import (
	"fmt"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/basefunction"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/rpc"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/state"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
	"log"
)

const FUNCTION_NAME = "input"
const FUNCTION_VERSION = "1.0.0"
const FUNCTION_DESCRIPTION = `
Takes an input as a text and returns the same text as output.
`

type Inputs struct {
	Text string `json:"text"`
}

type Outputs struct {
	Output string `json:"output"`
}

func function(inputs Inputs, _ *types.EventMessage, _ *rpc.RpcClient) (Outputs, error) {
	log.Println("Starting input function")
	if inputs.Text == "error" {
		return Outputs{}, fmt.Errorf("test error: intentionally throwing an error")
	}

	log.Println("Printing text: ", inputs.Text)
	log.Println("Finished input function")

	// Wait for 3 seconds before returning output
	// time.Sleep(3 * time.Second)

	return Outputs{
		Output: inputs.Text,
	}, nil
}

func NewFunction(gs *state.GlobalState) basefunction.FunctionInterface {
	return basefunction.NewFunction(
		FUNCTION_NAME,
		FUNCTION_VERSION,
		FUNCTION_DESCRIPTION,
		func(inputs Inputs, eventState *types.EventMessage) (Outputs, error) {
			return function(inputs, eventState, gs.RpcClient)
		},
		[]string{},
	)
}
