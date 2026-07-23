package state

import (
	"github.com/dibbla-agents/sdk-go/internal/basefunction"
	"github.com/dibbla-agents/sdk-go/internal/communication"
	"github.com/dibbla-agents/sdk-go/internal/dispatcher"
	"github.com/dibbla-agents/sdk-go/internal/grpccache"
	"github.com/dibbla-agents/sdk-go/internal/grpcstore"
	"github.com/dibbla-agents/sdk-go/internal/maps"
	"github.com/dibbla-agents/sdk-go/internal/oauth"
	"github.com/dibbla-agents/sdk-go/internal/rpc"
	"github.com/dibbla-agents/sdk-go/internal/types"
)

type GlobalState struct {
	GrpcCache        *grpccache.Client
	GrpcStore        *grpcstore.Client
	OAuth            *oauth.Client
	ServerName       string
	Functions        *maps.SafeFunctionMap[string, basefunction.FunctionInterface]
	ResponseHandlers *maps.SafeFunctionMap[string, chan *[]byte]
	RpcClient        *rpc.RpcClient
	ExecutionState   *maps.SafeFunctionMap[string, any]
	WorkflowComm     communication.WorkflowCommunicator
	Dispatcher       *dispatcher.Dispatcher
	// CapabilityProviders holds the provider definitions announced to the
	// workflow server alongside functions. Populated once during
	// Server.Start() before handlers activate; read-only afterwards.
	CapabilityProviders []types.CapabilityProviderDefinition
}
