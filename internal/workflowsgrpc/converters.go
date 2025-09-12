package workflowsgrpc

import (
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/types"

	"google.golang.org/protobuf/types/known/structpb"
)

// ConvertToGRPC converts a types.EventMessage to a GrpcEventMessage
func ConvertToGRPC(in *types.EventMessage) (*GrpcEventMessage, error) {
	meta := &structpb.Struct{}
	if in.Meta != nil {
		var err error
		meta, err = structpb.NewStruct(*in.Meta)
		if err != nil {
			return nil, err
		}
	}

	return &GrpcEventMessage{
		Function:      in.Function,
		Node:          in.Node,
		Workflow:      in.Workflow,
		Version:       in.Version,
		Server:        in.Server,
		Event:         in.Event,
		Text:          in.Text,
		Run:           in.Run,
		Meta:          meta,
		Payload:       derefBytes(in.Payload),
		CorrelationId: in.CorrelationID,
	}, nil
}

// ConvertFromGRPC converts a GrpcEventMessage to a types.EventMessage
func ConvertFromGRPC(in *GrpcEventMessage) (*types.EventMessage, error) {
	var meta *map[string]any
	if in.Meta != nil {
		metaMap := in.Meta.AsMap()
		meta = &metaMap
	}

	var payload *[]byte
	if len(in.Payload) > 0 {
		payloadCopy := make([]byte, len(in.Payload))
		copy(payloadCopy, in.Payload)
		payload = &payloadCopy
	}

	return &types.EventMessage{
		Function:      in.Function,
		Node:          in.Node,
		Workflow:      in.Workflow,
		Version:       in.Version,
		Server:        in.Server,
		Event:         in.Event,
		Text:          in.Text,
		Run:           in.Run,
		Meta:          meta,
		Payload:       payload,
		CorrelationID: in.CorrelationId,
	}, nil
}

// derefBytes safely dereferences a *[]byte to []byte
func derefBytes(b *[]byte) []byte {
	if b == nil {
		return nil
	}
	return *b
}
