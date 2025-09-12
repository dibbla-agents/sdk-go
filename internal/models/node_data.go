package models

import (
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/internal/basefunction"

	"github.com/tmc/langchaingo/llms"
)

type Workflow struct {
	Edges    []Edge   `json:"edges"`
	Nodes    []Node   `json:"nodes"`
	Name     string   `json:"name,omitempty"` // Added for Redis storage
	Metadata Metadata `json:"metadata"`       // New field added
}

// Metadata represents additional information about the workflow
type Metadata struct {
	WorkflowName string `json:"workflowName"`
}
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Edge represents a connection between nodes
type Edge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	SourceHandle string `json:"sourceHandle"`
	Target       string `json:"target"`
	TargetHandle string `json:"targetHandle"`
}

// Node represents a node in the workflow
type Node struct {
	Data             NodeData  `json:"data"`
	Dragging         bool      `json:"dragging,omitempty"`
	Height           int       `json:"height,omitempty"`
	ID               string    `json:"id"`
	Position         Position  `json:"position"`
	PositionAbsolute *Position `json:"positionAbsolute,omitempty"`
	Selected         bool      `json:"selected,omitempty"`
	Type             string    `json:"type"`
	Width            int       `json:"width,omitempty"`
}

type ToolDefinition struct {
	Type     string           `json:"type"`
	Function FunctionToolSpec `json:"function"`
}

// ToLLMSTool converts a ToolDefinition to llms.Tool
func (t *ToolDefinition) ToLLMSTool() *llms.Tool {
	return &llms.Tool{
		Type: t.Type,
		Function: &llms.FunctionDefinition{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
			// Strict is defaulted to false
		},
	}
}

// FunctionToolSpec represents the function specification in a tool
type FunctionToolSpec struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Parameters  ParameterDefinition `json:"parameters"`
}

// ParameterDefinition represents the parameters object in a function tool
type ParameterDefinition struct {
	Type       string                     `json:"type"`
	Properties map[string]PropertyDetails `json:"properties"`
	Required   []string                   `json:"required,omitempty"`
}

// PropertyDetails represents the details of each property in parameters
type PropertyDetails struct {
	Type        string     `json:"type"`
	Description string     `json:"description,omitempty"`
	Enum        []string   `json:"enum,omitempty"`
	Items       *ItemsSpec `json:"items,omitempty"`
}

// ItemsSpec represents the items specification for array types
type ItemsSpec struct {
	Type string   `json:"type"`
	Enum []string `json:"enum,omitempty"`
}

// NodeData contains the main configuration of a node
type NodeData struct {
	ConnectedInputs map[string]bool                 `json:"connectedInputs"`
	WorkflowName    string                          `json:"workflowName"`
	Description     string                          `json:"description"`
	Function        basefunction.FunctionDefinition `json:"function"`
	Inputs          []NodeField                     `json:"inputs"`
	Label           string                          `json:"label"`
	Outputs         []NodeField                     `json:"outputs"`
	Tool            *ToolDefinition                 `json:"tool,omitempty"` // Added tool field
}

// NodeField represents input or output configuration
type NodeField struct {
	ID           string        `json:"id"`
	Value        interface{}   `json:"value,omitempty"`
	DefaultValue interface{}   `json:"defaultValue,omitempty"`
	Name         string        `json:"name"`
	Type         string        `json:"type"`
	UIType       string        `json:"ui_type"`
	Validation   *IOValidation `json:"validation,omitempty"`
}

// IOValidation contains validation rules for inputs
type IOValidation struct {
	Max float64 `json:"max,omitempty"`
	Min float64 `json:"min,omitempty"`
}
