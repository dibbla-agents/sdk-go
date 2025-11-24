package functions

import (
	"github.com/dibbla-agents/sdk-go/internal/basefunction"
	"github.com/dibbla-agents/sdk-go/internal/state"
	"github.com/dibbla-agents/sdk-go/internal/types"
)

func GetFunction(gs *state.GlobalState, functionName string, version string) (basefunction.FunctionInterface, bool) {
	identifier := types.FunctionKey(gs.ServerName, functionName, version)
	return gs.Functions.Load(identifier)
}

func GetIdentifier(functionName string, version string) string { return functionName + "|" + version }
