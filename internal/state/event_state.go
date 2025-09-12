package state

type EventState struct {
	Server         string
	Function       string
	FunctionServer string
	Node           string
	Workflow       string
	Version        string
	Run            string
	CorrelationID  string
}

func NewEventState(server string, function string, version string, node string, workflow string, run string, functionServer string, correlationID string) *EventState {
	return &EventState{
		Server:         server,
		Function:       function,
		FunctionServer: functionServer,
		Node:           node,
		Workflow:       workflow,
		Version:        version,
		Run:            run,
		CorrelationID:  correlationID,
	}
}
