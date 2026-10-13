package state

import (
	"os"
	"testing"
)

// NewGlobalStateFromEnvironment must resolve the server exactly as sdk.New
// does, platform fallback included.
func TestCommunicationConfigFromEnvironment(t *testing.T) {
	for _, k := range []string{"GRPC_SERVER_ADDRESS", "GRPC_USE_TLS", "DIBBLA_GRPC_SERVER_ADDRESS", "DIBBLA_GRPC_USE_TLS"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}

	if got := communicationConfigFromEnvironment(); got.GrpcServerAddress != "grpc.dibbla.com:443" || got.UseTLS != nil {
		t.Fatalf("defaults: address %q, UseTLS %v", got.GrpcServerAddress, got.UseTLS)
	}

	t.Setenv("DIBBLA_GRPC_SERVER_ADDRESS", "dibbla-grpc.internal:50051")
	t.Setenv("DIBBLA_GRPC_USE_TLS", "false")
	got := communicationConfigFromEnvironment()
	if got.GrpcServerAddress != "dibbla-grpc.internal:50051" {
		t.Errorf("platform address: got %q", got.GrpcServerAddress)
	}
	if got.UseTLS == nil || *got.UseTLS {
		t.Errorf("platform TLS hint false: got %v", got.UseTLS)
	}

	t.Setenv("GRPC_SERVER_ADDRESS", "grpc.other.example.com:443")
	got = communicationConfigFromEnvironment()
	if got.GrpcServerAddress != "grpc.other.example.com:443" {
		t.Errorf("GRPC_SERVER_ADDRESS must win: got %q", got.GrpcServerAddress)
	}
	if got.UseTLS != nil {
		t.Errorf("platform TLS hint must not apply to the user's address: got %v", *got.UseTLS)
	}

	t.Setenv("GRPC_USE_TLS", "true")
	if got := communicationConfigFromEnvironment(); got.UseTLS == nil || !*got.UseTLS {
		t.Errorf("GRPC_USE_TLS=true: got %v", got.UseTLS)
	}
}
