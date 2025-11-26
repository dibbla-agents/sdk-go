package examples

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/dibbla-agents/sdk-go"
	"github.com/dibbla-agents/sdk-go/internal/oauth"
	"github.com/dibbla-agents/sdk-go/internal/state"
	"github.com/dibbla-agents/sdk-go/internal/types"
)

// GetGoogleTokenInputs defines the input for the GetGoogleToken function
type GetGoogleTokenInputs struct{}

// GetGoogleTokenOutputs defines the output for the GetGoogleToken function
type GetGoogleTokenOutputs struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresAt   int64  `json:"expires_at"`
	Provider    string `json:"provider"`
}

// GetGoogleTokenFunction demonstrates how to get a Google OAuth access token
// for the user running the workflow.
func GetGoogleTokenFunction() sdk.FunctionBuilder {
	return sdk.NewFunction[GetGoogleTokenInputs, GetGoogleTokenOutputs](
		"get_google_token",
		"1.0.0",
		"Gets a Google OAuth access token for the current user. Use this token to call Google APIs on behalf of the user.",
	).WithHandler(func(inputs GetGoogleTokenInputs, event *types.EventMessage, gs *state.GlobalState) (GetGoogleTokenOutputs, error) {
		if gs.OAuth == nil {
			return GetGoogleTokenOutputs{}, fmt.Errorf("OAuth client not available")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		token, err := gs.OAuth.GetAccessToken(ctx, oauth.ProviderGoogle, event.Run)
		if err != nil {
			return GetGoogleTokenOutputs{}, fmt.Errorf("failed to get Google token: %w", err)
		}

		return GetGoogleTokenOutputs{
			AccessToken: token.AccessToken,
			TokenType:   token.TokenType,
			ExpiresAt:   token.ExpiresAt,
			Provider:    token.Provider,
		}, nil
	}).WithTags("oauth", "google", "authentication")
}

// CheckConnectedProvidersInputs defines the input for the CheckConnectedProviders function
type CheckConnectedProvidersInputs struct{}

// ProviderInfo contains information about a connected OAuth provider
type ProviderInfo struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	LastUsed *int64 `json:"last_used,omitempty"`
	Scopes   string `json:"scopes"`
}

// CheckConnectedProvidersOutputs defines the output for the CheckConnectedProviders function
type CheckConnectedProvidersOutputs struct {
	Providers []ProviderInfo `json:"providers"`
}

// CheckConnectedProvidersFunction demonstrates how to check which OAuth providers
// the user has connected.
func CheckConnectedProvidersFunction() sdk.FunctionBuilder {
	return sdk.NewFunction[CheckConnectedProvidersInputs, CheckConnectedProvidersOutputs](
		"check_connected_providers",
		"1.0.0",
		"Checks which OAuth providers (Google, Microsoft, GitHub) the current user has connected.",
	).WithHandler(func(inputs CheckConnectedProvidersInputs, event *types.EventMessage, gs *state.GlobalState) (CheckConnectedProvidersOutputs, error) {
		if gs.OAuth == nil {
			return CheckConnectedProvidersOutputs{}, fmt.Errorf("OAuth client not available")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		providers, err := gs.OAuth.GetConnectedProviders(ctx, event.Run)
		if err != nil {
			return CheckConnectedProvidersOutputs{}, fmt.Errorf("failed to check connected providers: %w", err)
		}

		// Convert to output format
		result := make([]ProviderInfo, 0, len(providers))
		for name, status := range providers {
			result = append(result, ProviderInfo{
				Name:     name,
				Email:    status.Email,
				LastUsed: status.LastUsed,
				Scopes:   status.Scopes,
			})
		}

		return CheckConnectedProvidersOutputs{Providers: result}, nil
	}).WithTags("oauth", "authentication", "status")
}
