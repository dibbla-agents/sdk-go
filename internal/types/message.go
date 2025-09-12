package types

type EventMessage struct {
	Function      string          `json:"function"`
	Node          string          `json:"node"`
	Workflow      string          `json:"workflow"`
	Version       string          `json:"version"`
	Server        string          `json:"server"`
	Event         string          `json:"event"`
	Text          string          `json:"text"`
	Run           string          `json:"run"`
	Meta          *map[string]any `json:"meta"`
	Payload       *[]byte         `json:"payload"`
	CorrelationID string          `json:"correlation_id"`
}
