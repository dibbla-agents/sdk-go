package communication

import (
	"os"
	"testing"
)

// unsetEndpointEnv clears every variable EndpointFromEnv reads, restoring
// them when the test ends.
func unsetEndpointEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{ServerAddressEnv, UseTLSEnv, PlatformServerAddressEnv, PlatformUseTLSEnv} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func boolp(b bool) *bool { return &b }

func tlsString(b *bool) string {
	switch {
	case b == nil:
		return "nil"
	case *b:
		return "true"
	default:
		return "false"
	}
}

func TestEndpointFromEnvPrecedence(t *testing.T) {
	cases := []struct {
		name         string
		env          map[string]string
		wantAddress  string
		wantPlatform bool
		wantTLS      *bool // EnvEndpoint.TLS()
	}{
		{
			name:        "neither set: hosted default, auto-detect",
			wantAddress: DefaultServerAddress,
		},
		{
			name: "GRPC_SERVER_ADDRESS wins over the platform address",
			env: map[string]string{
				ServerAddressEnv:         "localhost:50051",
				PlatformServerAddressEnv: "grpc.dibbla-dev.example.com:443",
			},
			wantAddress: "localhost:50051",
		},
		{
			name:         "platform address when GRPC_SERVER_ADDRESS is unset",
			env:          map[string]string{PlatformServerAddressEnv: "grpc.dibbla-dev.example.com:443"},
			wantAddress:  "grpc.dibbla-dev.example.com:443",
			wantPlatform: true,
		},
		{
			name: "platform address when GRPC_SERVER_ADDRESS is blank",
			env: map[string]string{
				ServerAddressEnv:         "  ",
				PlatformServerAddressEnv: " grpc.dibbla-dev.example.com:443 ",
			},
			wantAddress:  "grpc.dibbla-dev.example.com:443",
			wantPlatform: true,
		},
		{
			name:        "blank platform address falls through to the default",
			env:         map[string]string{PlatformServerAddressEnv: " ", PlatformUseTLSEnv: "false"},
			wantAddress: DefaultServerAddress,
		},
		{
			name: "platform TLS hint applies to the platform address",
			env: map[string]string{
				PlatformServerAddressEnv: "dibbla-grpc.internal:50051",
				PlatformUseTLSEnv:        "false",
			},
			wantAddress:  "dibbla-grpc.internal:50051",
			wantPlatform: true,
			wantTLS:      boolp(false),
		},
		{
			name: "platform TLS hint does not apply to the user's address",
			env: map[string]string{
				ServerAddressEnv:         "grpc.other.example.com:443",
				PlatformServerAddressEnv: "dibbla-grpc.internal:50051",
				PlatformUseTLSEnv:        "false",
			},
			wantAddress: "grpc.other.example.com:443",
		},
		{
			name:        "platform TLS hint without a platform address is ignored",
			env:         map[string]string{PlatformUseTLSEnv: "false"},
			wantAddress: DefaultServerAddress,
		},
		{
			name: "GRPC_USE_TLS beats the platform TLS hint",
			env: map[string]string{
				UseTLSEnv:                "true",
				PlatformServerAddressEnv: "dibbla-grpc.internal:50051",
				PlatformUseTLSEnv:        "false",
			},
			wantAddress:  "dibbla-grpc.internal:50051",
			wantPlatform: true,
			wantTLS:      boolp(true),
		},
		{
			name:        "GRPC_USE_TLS keeps its reading: non-empty and not true/1 is false",
			env:         map[string]string{UseTLSEnv: "no"},
			wantAddress: DefaultServerAddress,
			wantTLS:     boolp(false),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			unsetEndpointEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			e := EndpointFromEnv()
			if e.Address != tc.wantAddress {
				t.Errorf("Address = %q, want %q", e.Address, tc.wantAddress)
			}
			if e.FromPlatform != tc.wantPlatform {
				t.Errorf("FromPlatform = %v, want %v", e.FromPlatform, tc.wantPlatform)
			}
			if got, want := tlsString(e.TLS()), tlsString(tc.wantTLS); got != want {
				t.Errorf("TLS() = %s, want %s", got, want)
			}
		})
	}
}

func TestPlatformTLSHintParsing(t *testing.T) {
	cases := map[string]*bool{
		"true":   boolp(true),
		"1":      boolp(true),
		" true ": boolp(true),
		"false":  boolp(false),
		"0":      boolp(false),
		"":       nil,
		"yes":    nil,
		"TRUE":   nil,
	}
	for value, want := range cases {
		unsetEndpointEnv(t)
		t.Setenv(PlatformServerAddressEnv, "dibbla-grpc.internal:50051")
		t.Setenv(PlatformUseTLSEnv, value)
		if got := tlsString(EndpointFromEnv().PlatformUseTLS); got != tlsString(want) {
			t.Errorf("DIBBLA_GRPC_USE_TLS=%q: PlatformUseTLS = %s, want %s", value, got, tlsString(want))
		}
	}
}
