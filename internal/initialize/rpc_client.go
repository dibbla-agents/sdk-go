package initialize

import (
	"github.com/dibbla-agents/sdk-go/internal/rpc"
	"github.com/dibbla-agents/sdk-go/internal/types"
)

func RpcClient(workflowOut chan types.EventMessage) *rpc.RpcClient {
	return rpc.NewRpcClient(workflowOut)
}
