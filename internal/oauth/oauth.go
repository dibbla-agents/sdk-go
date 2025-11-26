package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dibbla-agents/sdk-go/internal/communication"
	"github.com/dibbla-agents/sdk-go/internal/correlation"
	"github.com/dibbla-agents/sdk-go/internal/types"
	"github.com/dibbla-agents/sdk-go/internal/utils"
)

// Provider represents supported OAuth providers
type Provider string

const (
	ProviderGoogle    Provider = "google"
	ProviderMicrosoft Provider = "microsoft"
	ProviderGitHub    Provider = "github"
)

// TokenResponse represents a successful OAuth token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresAt   int64  `json:"expires_at"`
	Provider    string `json:"provider"`
}

// ProviderStatus represents OAuth connection status for a provider
type ProviderStatus struct {
	Connected bool   `json:"connected"`
	Email     string `json:"email"`
	LastUsed  *int64 `json:"last_used"`
	Scopes    string `json:"scopes"`
}

// OAuthError represents an OAuth error from the server
type OAuthError struct {
	Code    string `json:"error"`
	Message string `json:"error_message"`
}

func (e *OAuthError) Error() string {
	return fmt.Sprintf("oauth error [%s]: %s", e.Code, e.Message)
}

// Client provides OAuth token operations over the workflow gRPC stream
type Client struct {
	communicator   communication.WorkflowCommunicator
	router         *correlation.Router
	defaultTimeout time.Duration
	serverName     string
}

// NewClient creates a new OAuth client
func NewClient(comm communication.WorkflowCommunicator, serverName string) *Client {
	return &Client{
		communicator:   comm,
		router:         correlation.NewRouter(),
		defaultTimeout: 30 * time.Second,
		serverName:     serverName,
	}
}

// GetAccessToken requests an OAuth access token for the specified provider.
// Uses the run_id to resolve organization context automatically.
func (c *Client) GetAccessToken(ctx context.Context, provider Provider, runID string) (*TokenResponse, error) {
	if c == nil || c.communicator == nil {
		return nil, fmt.Errorf("oauth: no communicator configured")
	}

	correlationID := utils.UID()
	responseChan := c.router.Register(correlationID, 1)
	defer c.router.Remove(correlationID)

	payload := map[string]string{
		"provider": string(provider),
		"run_id":   runID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("oauth: failed to marshal request: %w", err)
	}

	event := types.EventMessage{
		Run:           runID,
		Event:         types.EventOAuthTokenRequest,
		Text:          "OAuth token request",
		Payload:       &payloadBytes,
		CorrelationID: correlationID,
	}

	if err := c.communicator.SendEvent(&event); err != nil {
		return nil, fmt.Errorf("oauth: failed to send token request: %w", err)
	}

	select {
	case resp := <-responseChan:
		return c.parseTokenResponse(&resp)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetAccessTokenWithTimeout requests an OAuth access token using the client's default timeout
func (c *Client) GetAccessTokenWithTimeout(provider Provider, runID string) (*TokenResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.defaultTimeout)
	defer cancel()
	return c.GetAccessToken(ctx, provider, runID)
}

// GetConnectedProviders checks which OAuth providers are connected for the current run.
// Returns a map of provider name to status (only connected providers are included).
func (c *Client) GetConnectedProviders(ctx context.Context, runID string) (map[string]*ProviderStatus, error) {
	if c == nil || c.communicator == nil {
		return nil, fmt.Errorf("oauth: no communicator configured")
	}

	correlationID := utils.UID()
	responseChan := c.router.Register(correlationID, 1)
	defer c.router.Remove(correlationID)

	payload := map[string]string{"run_id": runID}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("oauth: failed to marshal request: %w", err)
	}

	event := types.EventMessage{
		Run:           runID,
		Event:         types.EventOAuthStatusRequest,
		Text:          "OAuth status request",
		Payload:       &payloadBytes,
		CorrelationID: correlationID,
	}

	if err := c.communicator.SendEvent(&event); err != nil {
		return nil, fmt.Errorf("oauth: failed to send status request: %w", err)
	}

	select {
	case resp := <-responseChan:
		return c.parseStatusResponse(&resp)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetConnectedProvidersWithTimeout checks connected providers using the client's default timeout
func (c *Client) GetConnectedProvidersWithTimeout(runID string) (map[string]*ProviderStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.defaultTimeout)
	defer cancel()
	return c.GetConnectedProviders(ctx, runID)
}

// IsProviderConnected is a convenience method to check if a specific provider is connected
func (c *Client) IsProviderConnected(ctx context.Context, provider Provider, runID string) (bool, error) {
	providers, err := c.GetConnectedProviders(ctx, runID)
	if err != nil {
		return false, err
	}
	_, connected := providers[string(provider)]
	return connected, nil
}

func (c *Client) parseTokenResponse(resp *types.EventMessage) (*TokenResponse, error) {
	if resp.Event == types.EventOAuthError {
		return nil, c.parseError(resp)
	}
	if resp.Payload == nil {
		return nil, fmt.Errorf("oauth: empty response payload")
	}
	var token TokenResponse
	if err := json.Unmarshal(*resp.Payload, &token); err != nil {
		return nil, fmt.Errorf("oauth: failed to parse token response: %w", err)
	}
	return &token, nil
}

func (c *Client) parseStatusResponse(resp *types.EventMessage) (map[string]*ProviderStatus, error) {
	if resp.Event == types.EventOAuthError {
		return nil, c.parseError(resp)
	}
	if resp.Payload == nil {
		return nil, fmt.Errorf("oauth: empty response payload")
	}
	var status map[string]*ProviderStatus
	if err := json.Unmarshal(*resp.Payload, &status); err != nil {
		return nil, fmt.Errorf("oauth: failed to parse status response: %w", err)
	}
	return status, nil
}

func (c *Client) parseError(resp *types.EventMessage) error {
	if resp.Payload == nil {
		return fmt.Errorf("oauth: unknown error")
	}
	var oauthErr OAuthError
	if err := json.Unmarshal(*resp.Payload, &oauthErr); err != nil {
		return fmt.Errorf("oauth: error: %s", string(*resp.Payload))
	}
	return &oauthErr
}

// HandleResponse delivers OAuth response events to waiting goroutines using the correlation ID.
// Supported events: oauth_token_response, oauth_status_response, oauth_error
func (c *Client) HandleResponse(response types.EventMessage) {
	if c == nil {
		return
	}
	switch response.Event {
	case types.EventOAuthTokenResponse, types.EventOAuthStatusResponse, types.EventOAuthError:
		c.router.Deliver(response.CorrelationID, response)
	}
}

