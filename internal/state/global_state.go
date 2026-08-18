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
	// CapabilityProviderHandlers maps "<capability>/<name>" to the invoke
	// closure that decodes a capability_provider_request, runs the typed
	// seat handler (e.g. ToolSearchProvider.Select), and encodes the
	// response payload (DIB-152). Populated alongside CapabilityProviders;
	// read-only after handlers activate. Absent key = no handler declared
	// (the request gets an explicit "not supported" error).
	CapabilityProviderHandlers map[string]CapabilityProviderHandler
}

// CapabilityProviderHandler decodes a capability_provider_request payload,
// runs the provider's seat logic, and returns the response payload to send
// back. A non-nil error is turned into an error event by the dispatcher.
type CapabilityProviderHandler func(reqPayload *[]byte) (*[]byte, error)
