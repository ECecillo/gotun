//go:build darwin && integration

package darwin_test

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"

	"ececillo/gotun/pkg/tunnel/darwin"

	"golang.org/x/sys/unix"
)

func TestInterfaceConfigurator_ConfigureInterface_Integration(t *testing.T) {
	skipIfNotRoot(t)

	// First, create a UTUN interface to configure
	tunName := createTestTunnel(t)

	// Create interface configurator
	configurator, err := darwin.NewInterfaceConfigurator()
	if err != nil {
		t.Fatalf("failed to create interface configurator: %v", err)
	}
	defer func() {
		_ = configurator.Close()
	}()

	// Configure the interface
	sourceIP := net.ParseIP("10.0.88.2").To4()
	destIP := net.ParseIP("10.0.88.1").To4()

	err = configurator.ConfigureInterface(tunName, sourceIP, destIP)
	if err != nil {
		t.Fatalf("failed to configure interface: %v", err)
	}

	// Verify the configuration
	verifyIPConfiguration(t, tunName, sourceIP, destIP)
	verifyInterfaceFlags(t, tunName, true, true) // UP and RUNNING

	t.Logf("Successfully configured interface %s", tunName)
}

func TestInterfaceConfigurator_Lifecycle_Integration(t *testing.T) {
	skipIfNotRoot(t)

	// Test creating and closing configurator multiple times
	for i := 0; i < 3; i++ {
		configurator, err := darwin.NewInterfaceConfigurator()
		if err != nil {
			t.Fatalf("iteration %d: failed to create configurator: %v", i, err)
		}

		if err := configurator.Close(); err != nil {
			t.Errorf("iteration %d: failed to close configurator: %v", i, err)
		}

		t.Logf("Iteration %d: successfully created and closed configurator", i)
	}
}

func TestInterfaceConfigurator_MultipleInterfaces_Integration(t *testing.T) {
	skipIfNotRoot(t)

	// Create multiple tunnels and configure them
	numTunnels := 3
	tunnels := make([]string, numTunnels)

	// Create tunnels
	for i := 0; i < numTunnels; i++ {
		tunnels[i] = createTestTunnel(t)
		t.Logf("Created tunnel %d: %s", i, tunnels[i])
	}

	// Configure all tunnels
	configurator, err := darwin.NewInterfaceConfigurator()
	if err != nil {
		t.Fatalf("failed to create configurator: %v", err)
	}
	defer func() {
		_ = configurator.Close()
	}()

	for i, tunName := range tunnels {
		sourceIP := net.ParseIP(fmt.Sprintf("10.0.%d.2", 80+i)).To4()
		destIP := net.ParseIP(fmt.Sprintf("10.0.%d.1", 80+i)).To4()

		err := configurator.ConfigureInterface(tunName, sourceIP, destIP)
		if err != nil {
			t.Errorf("failed to configure tunnel %d (%s): %v", i, tunName, err)
			continue
		}

		verifyIPConfiguration(t, tunName, sourceIP, destIP)
		t.Logf("Configured tunnel %d: %s", i, tunName)
	}
}

func TestInterfaceConfigurator_DifferentIPRanges_Integration(t *testing.T) {
	skipIfNotRoot(t)

	tests := []struct {
		name      string
		sourceIP  string
		destIP    string
		shouldErr bool
	}{
		{
			name:      "10.x network",
			sourceIP:  "10.1.2.3",
			destIP:    "10.1.2.1",
			shouldErr: false,
		},
		{
			name:      "172.x network",
			sourceIP:  "172.16.0.2",
			destIP:    "172.16.0.1",
			shouldErr: false,
		},
		{
			name:      "192.168.x network",
			sourceIP:  "192.168.1.2",
			destIP:    "192.168.1.1",
			shouldErr: false,
		},
	}

	configurator, err := darwin.NewInterfaceConfigurator()
	if err != nil {
		t.Fatalf("failed to create configurator: %v", err)
	}
	defer func() {
		_ = configurator.Close()
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new tunnel for this test
			tunName := createTestTunnel(t)

			sourceIP := net.ParseIP(tt.sourceIP).To4()
			destIP := net.ParseIP(tt.destIP).To4()

			err := configurator.ConfigureInterface(tunName, sourceIP, destIP)
			if (err != nil) != tt.shouldErr {
				t.Errorf("ConfigureInterface() error = %v, wantErr %v", err, tt.shouldErr)
				return
			}

			if !tt.shouldErr {
				verifyIPConfiguration(t, tunName, sourceIP, destIP)
			}
		})
	}
}

// Helper functions

func createTestTunnel(t *testing.T) string {
	t.Helper()

	// Open kernel control socket
	fd, err := darwin.OpenKernelControlSocket()
	if err != nil {
		t.Fatalf("failed to open kernel control socket: %v", err)
	}

	// Get controller info
	info, err := darwin.RetrieveUTUNControllerID(fd)
	if err != nil {
		_ = unix.Close(fd)
		t.Fatalf("failed to get controller ID: %v", err)
	}

	// Connect to controller
	if err := darwin.ConnectToUTUNController(fd, info); err != nil {
		_ = unix.Close(fd)
		t.Fatalf("failed to connect to controller: %v", err)
	}

	// Get interface name
	name, err := darwin.GetUTUNInterfaceName(fd)
	if err != nil {
		_ = unix.Close(fd)
		t.Fatalf("failed to get interface name: %v", err)
	}

	// Note: We keep the fd open - it will be cleaned up by the test framework
	// or when the process exits. This is intentional for integration tests.

	return name
}

func verifyIPConfiguration(t *testing.T, tunName string, sourceIP, destIP net.IP) {
	t.Helper()

	cmd := exec.Command("ifconfig", tunName)
	output, err := cmd.Output()
	if err != nil {
		t.Errorf("failed to get interface config: %v", err)
		return
	}

	outputStr := string(output)

	// Verify source IP
	if !strings.Contains(outputStr, sourceIP.String()) {
		t.Errorf("source IP %s not found in ifconfig output:\n%s", sourceIP, outputStr)
	}

	// Verify destination IP
	if !strings.Contains(outputStr, destIP.String()) {
		t.Errorf("destination IP %s not found in ifconfig output:\n%s", destIP, outputStr)
	}
}

func verifyInterfaceFlags(t *testing.T, tunName string, wantUp, wantRunning bool) {
	t.Helper()

	cmd := exec.Command("ifconfig", tunName)
	output, err := cmd.Output()
	if err != nil {
		t.Errorf("failed to get interface config: %v", err)
		return
	}

	outputStr := string(output)

	// Check for UP flag
	hasUp := strings.Contains(outputStr, "UP")
	if hasUp != wantUp {
		t.Errorf("interface UP flag: got %v, want %v", hasUp, wantUp)
	}

	// Check for RUNNING flag
	hasRunning := strings.Contains(outputStr, "RUNNING")
	if hasRunning != wantRunning {
		t.Errorf("interface RUNNING flag: got %v, want %v", hasRunning, wantRunning)
	}
}
