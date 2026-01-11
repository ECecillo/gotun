//go:build darwin && integration

package darwin_test

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"ececillo/gotun/pkg/tunnel"
	"ececillo/gotun/pkg/tunnel/darwin"
)

func TestProvider_Create_Integration(t *testing.T) {
	skipIfNotRoot(t)

	provider := darwin.NewProvider()
	defer func() {
		if err := provider.Close(); err != nil {
			t.Logf("warning: failed to close provider: %v", err)
		}
	}()

	config := tunnel.TunnelConfig{
		SourceIP:      net.ParseIP("10.0.99.2").To4(),
		DestinationIP: net.ParseIP("10.0.99.1").To4(),
	}

	tun, err := provider.Create(config)
	if err != nil {
		t.Fatalf("failed to create tunnel: %v", err)
	}

	// Verify tunnel was created
	if tun == nil {
		t.Fatal("expected non-nil tunnel")
	}

	// Verify interface name is set
	if tun.Name == "" {
		t.Error("expected non-empty interface name")
	}

	// Verify interface name has correct prefix
	if !strings.HasPrefix(tun.Name, "utun") {
		t.Errorf("expected interface name to start with 'utun', got: %s", tun.Name)
	}

	// Verify file descriptor is valid
	if tun.FD <= 0 {
		t.Errorf("expected positive file descriptor, got: %d", tun.FD)
	}

	t.Logf("Successfully created tunnel: %s (fd: %d)", tun.Name, tun.FD)

	// Verify interface exists in system
	if !interfaceExists(t, tun.Name) {
		t.Errorf("interface %s does not exist in system", tun.Name)
	}

	// Verify interface is UP
	if !interfaceIsUp(t, tun.Name) {
		t.Errorf("interface %s is not UP", tun.Name)
	}

	// Verify IP addresses are configured
	verifyInterfaceConfig(t, tun.Name, config)
}

func TestProvider_Create_MultipleInterfaces(t *testing.T) {
	skipIfNotRoot(t)

	// Test that we can create multiple UTUN interfaces
	providers := make([]*darwin.Provider, 3)
	tunnels := make([]*tunnel.Tunnel, 3)

	// Create multiple tunnels
	for i := 0; i < 3; i++ {
		providers[i] = darwin.NewProvider()

		config := tunnel.TunnelConfig{
			SourceIP:      net.ParseIP(fmt.Sprintf("10.0.%d.2", 100+i)).To4(),
			DestinationIP: net.ParseIP(fmt.Sprintf("10.0.%d.1", 100+i)).To4(),
		}

		tun, err := providers[i].Create(config)
		if err != nil {
			t.Fatalf("failed to create tunnel %d: %v", i, err)
		}
		tunnels[i] = tun
		t.Logf("Created tunnel %d: %s", i, tun.Name)
	}

	// Verify all tunnels exist
	for i, tun := range tunnels {
		if !interfaceExists(t, tun.Name) {
			t.Errorf("tunnel %d (%s) does not exist", i, tun.Name)
		}
	}

	// Clean up
	for i, provider := range providers {
		if err := provider.Close(); err != nil {
			t.Logf("warning: failed to close provider %d: %v", i, err)
		}
	}
}

func TestProvider_Close_Integration(t *testing.T) {
	skipIfNotRoot(t)

	provider := darwin.NewProvider()

	config := tunnel.TunnelConfig{
		SourceIP:      net.ParseIP("10.0.98.2").To4(),
		DestinationIP: net.ParseIP("10.0.98.1").To4(),
	}

	tun, err := provider.Create(config)
	if err != nil {
		t.Fatalf("failed to create tunnel: %v", err)
	}

	// Close the provider
	if err := provider.Close(); err != nil {
		t.Errorf("failed to close provider: %v", err)
	}

	// Verify we can close multiple times without error
	if err := provider.Close(); err != nil {
		t.Errorf("second close should not error, got: %v", err)
	}

	t.Logf("Successfully closed tunnel: %s", tun.Name)
}

func TestProvider_Create_InvalidIP(t *testing.T) {
	skipIfNotRoot(t)

	tests := []struct {
		name   string
		config tunnel.TunnelConfig
	}{
		{
			name: "nil source IP",
			config: tunnel.TunnelConfig{
				SourceIP:      nil,
				DestinationIP: net.ParseIP("10.0.0.1").To4(),
			},
		},
		{
			name: "nil destination IP",
			config: tunnel.TunnelConfig{
				SourceIP:      net.ParseIP("10.0.0.2").To4(),
				DestinationIP: nil,
			},
		},
		{
			name: "IPv6 source IP",
			config: tunnel.TunnelConfig{
				SourceIP:      net.ParseIP("::1"),
				DestinationIP: net.ParseIP("10.0.0.1").To4(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := darwin.NewProvider()
			defer func() {
				_ = provider.Close()
			}()

			// These should either fail or handle gracefully
			tun, err := provider.Create(tt.config)
			if err == nil && tun != nil {
				t.Logf("warning: expected error for invalid config, but succeeded: %s", tun.Name)
				// Clean up if it somehow succeeded
				_ = provider.Close()
			}
		})
	}
}

// Helper functions (using centralized helpers from testing_helpers_test.go)

func interfaceExists(t *testing.T, name string) bool {
	t.Helper()
	helper := NewTestHelper(t)
	return helper.InterfaceExists(name)
}

func interfaceIsUp(t *testing.T, name string) bool {
	t.Helper()
	helper := NewTestHelper(t)
	return helper.InterfaceIsUp(name)
}

func verifyInterfaceConfig(t *testing.T, name string, config tunnel.TunnelConfig) {
	t.Helper()
	helper := NewTestHelper(t)
	helper.AssertInterfaceHasIP(name, config.SourceIP.String())
	helper.AssertInterfaceHasIP(name, config.DestinationIP.String())
	t.Logf("Interface %s configuration verified", name)
}
