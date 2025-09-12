package initialize

import (
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/rpc"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"
)

func RpcClient(workflowOut chan types.EventMessage) *rpc.RpcClient {
	return rpc.NewRpcClient(workflowOut)
}
