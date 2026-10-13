package sdk

import (
	"os"
	"testing"
)

// grpcEnv sets the variables that name the workflow server; anything not
// in env is unset. Variables New writes back are restored too.
func grpcEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range []string{
		"GRPC_SERVER_ADDRESS", "GRPC_USE_TLS",
		"DIBBLA_GRPC_SERVER_ADDRESS", "DIBBLA_GRPC_USE_TLS",
		"SERVER_NAME", "CODEX_ENV_PATH",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func describeTLS(b *bool) string {
	switch {
	case b == nil:
		return "auto-detect"
	case *b:
		return "true"
	default:
		return "false"
	}
}

// connection is what the worker would dial: the address and TLS choice New
// hands to the communicator.
func connection(t *testing.T, opts ...Option) (string, string) {
	t.Helper()
	s, err := New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	cc := s.communicationConfig()
	return cc.GrpcServerAddress, describeTLS(cc.UseTLS)
}

const platformAddr = "dibbla-grpc.internal:50051"

func TestGrpcServerAddressPrecedence(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		opts     []Option
		wantAddr string
		wantTLS  string
	}{
		{
			name:     "neither set: hosted default",
			wantAddr: "grpc.dibbla.com:443",
			wantTLS:  "auto-detect",
		},
		{
			name:     "platform address when GRPC_SERVER_ADDRESS is unset",
			env:      map[string]string{"DIBBLA_GRPC_SERVER_ADDRESS": platformAddr},
			wantAddr: platformAddr,
			wantTLS:  "auto-detect",
		},
		{
			name:     "platform address when GRPC_SERVER_ADDRESS is empty",
			env:      map[string]string{"GRPC_SERVER_ADDRESS": "", "DIBBLA_GRPC_SERVER_ADDRESS": platformAddr},
			wantAddr: platformAddr,
			wantTLS:  "auto-detect",
		},
		{
			name:     "GRPC_SERVER_ADDRESS wins over the platform address",
			env:      map[string]string{"GRPC_SERVER_ADDRESS": "localhost:50051", "DIBBLA_GRPC_SERVER_ADDRESS": platformAddr},
			wantAddr: "localhost:50051",
			wantTLS:  "auto-detect",
		},
		{
			name: "WithGrpcServerAddress wins over both",
			env: map[string]string{
				"GRPC_SERVER_ADDRESS":        "localhost:50051",
				"DIBBLA_GRPC_SERVER_ADDRESS": platformAddr,
			},
			opts:     []Option{WithGrpcServerAddress("grpc.other.example.com:443")},
			wantAddr: "grpc.other.example.com:443",
			wantTLS:  "auto-detect",
		},
		{
			name: "WithGrpcMode wins over both",
			env: map[string]string{
				"GRPC_SERVER_ADDRESS":        "localhost:50051",
				"DIBBLA_GRPC_SERVER_ADDRESS": platformAddr,
			},
			opts:     []Option{WithGrpcMode("grpc.other.example.com:443")},
			wantAddr: "grpc.other.example.com:443",
			wantTLS:  "auto-detect",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			grpcEnv(t, tc.env)
			addr, tls := connection(t, tc.opts...)
			if addr != tc.wantAddr {
				t.Errorf("address = %q, want %q", addr, tc.wantAddr)
			}
			if tls != tc.wantTLS {
				t.Errorf("TLS = %s, want %s", tls, tc.wantTLS)
			}
		})
	}
}

func TestPlatformTLSHintAppliesOnlyToThePlatformAddress(t *testing.T) {
	platform := map[string]string{
		"DIBBLA_GRPC_SERVER_ADDRESS": platformAddr,
		"DIBBLA_GRPC_USE_TLS":        "false",
	}
	cases := []struct {
		name     string
		env      map[string]string
		opts     []Option
		wantAddr string
		wantTLS  string
	}{
		{
			name:     "platform address: the platform's hint",
			env:      platform,
			wantAddr: platformAddr,
			wantTLS:  "false",
		},
		{
			name:     "GRPC_SERVER_ADDRESS: the hint does not apply",
			env:      merge(platform, map[string]string{"GRPC_SERVER_ADDRESS": "grpc.other.example.com:443"}),
			wantAddr: "grpc.other.example.com:443",
			wantTLS:  "auto-detect",
		},
		{
			name:     "WithGrpcServerAddress: the hint does not apply",
			env:      platform,
			opts:     []Option{WithGrpcServerAddress("grpc.other.example.com:443")},
			wantAddr: "grpc.other.example.com:443",
			wantTLS:  "auto-detect",
		},
		{
			name:     "WithGrpcServerAddress naming the platform's own address: still the user's choice",
			env:      platform,
			opts:     []Option{WithGrpcServerAddress(platformAddr)},
			wantAddr: platformAddr,
			wantTLS:  "auto-detect",
		},
		{
			name:     "GRPC_USE_TLS beats the hint",
			env:      merge(platform, map[string]string{"GRPC_USE_TLS": "true"}),
			wantAddr: platformAddr,
			wantTLS:  "true",
		},
		{
			name:     "WithGrpcTLS beats GRPC_USE_TLS and the hint",
			env:      merge(platform, map[string]string{"GRPC_USE_TLS": "false", "DIBBLA_GRPC_USE_TLS": "false"}),
			opts:     []Option{WithGrpcTLS(true)},
			wantAddr: platformAddr,
			wantTLS:  "true",
		},
		{
			name:     "an unreadable hint is ignored",
			env:      merge(platform, map[string]string{"DIBBLA_GRPC_USE_TLS": "maybe"}),
			wantAddr: platformAddr,
			wantTLS:  "auto-detect",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			grpcEnv(t, tc.env)
			addr, tls := connection(t, tc.opts...)
			if addr != tc.wantAddr {
				t.Errorf("address = %q, want %q", addr, tc.wantAddr)
			}
			if tls != tc.wantTLS {
				t.Errorf("TLS = %s, want %s", tls, tc.wantTLS)
			}
		})
	}
}

// GetConfig hands out the live Config; an address set on it after New is
// the user's, so the platform's hint must not follow it.
func TestPlatformTLSHintDoesNotFollowAnAddressSetOnTheConfig(t *testing.T) {
	grpcEnv(t, map[string]string{
		"DIBBLA_GRPC_SERVER_ADDRESS": platformAddr,
		"DIBBLA_GRPC_USE_TLS":        "false",
	})
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	s.GetConfig().GrpcServerAddress = "grpc.other.example.com:443"
	if got := describeTLS(s.communicationConfig().UseTLS); got != "auto-detect" {
		t.Errorf("TLS = %s, want auto-detect", got)
	}
}

func merge(a, b map[string]string) map[string]string {
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
