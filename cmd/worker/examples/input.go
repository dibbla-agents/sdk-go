package examples

import (
	"fmt"

	sdk "github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/state"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
)

type inputExample struct {
}

type InputFunctionInputs struct {
	Text string `json:"text"`
}

type InputFunctionOutputs struct {
	Output string `json:"output"`
}

// InputFunction registers an example input function using the SDK
func InputFunction() sdk.FunctionBuilder {
	return sdk.NewFunction[InputFunctionInputs, InputFunctionOutputs](
		"example_input",
		"1.0.0",
		"Takes an input as a text and returns the same text as output.",
	).WithHandler(func(inputs InputFunctionInputs, _ *types.EventMessage, _ *state.GlobalState) (InputFunctionOutputs, error) {
		if inputs.Text == "error" {
			return InputFunctionOutputs{}, fmt.Errorf("test error: intentionally throwing an error")
		}
		return InputFunctionOutputs{Output: inputs.Text}, nil
	}).WithTags("utility", "input", "demo")
}
