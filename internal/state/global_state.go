package state

import (
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/basefunction"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/communication"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/dispatcher"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/grpccache"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/grpcstore"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/maps"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/rpc"
)

type GlobalState struct {
	GrpcCache        *grpccache.Client
	GrpcStore        *grpcstore.Client
	ServerName       string
	Functions        *maps.SafeFunctionMap[string, basefunction.FunctionInterface]
	ResponseHandlers *maps.SafeFunctionMap[string, chan *[]byte]
	RpcClient        *rpc.RpcClient
	ExecutionState   *maps.SafeFunctionMap[string, any]
	WorkflowComm     communication.WorkflowCommunicator
	Dispatcher       *dispatcher.Dispatcher
}
