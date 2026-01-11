//go:build darwin && integration

package darwin_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestHelper provides common utilities for integration tests.
type TestHelper struct {
	t *testing.T
}

// NewTestHelper creates a new test helper.
func NewTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{t: t}
}

// SkipIfNotRoot skips the test if not running with root privileges.
func (h *TestHelper) SkipIfNotRoot() {
	h.t.Helper()
	if os.Geteuid() != 0 {
		h.t.Skip("Skipping integration test: requires root privileges (run with sudo)")
	}
}

// InterfaceExists checks if a network interface exists.
func (h *TestHelper) InterfaceExists(name string) bool {
	h.t.Helper()
	cmd := exec.Command("ifconfig", name)
	err := cmd.Run()
	return err == nil
}

// InterfaceIsUp checks if a network interface has the UP flag set.
func (h *TestHelper) InterfaceIsUp(name string) bool {
	h.t.Helper()
	return h.interfaceHasFlag(name, "UP")
}

// InterfaceIsRunning checks if a network interface has the RUNNING flag set.
func (h *TestHelper) InterfaceIsRunning(name string) bool {
	h.t.Helper()
	return h.interfaceHasFlag(name, "RUNNING")
}

// interfaceHasFlag checks if an interface has a specific flag.
func (h *TestHelper) interfaceHasFlag(name, flag string) bool {
	h.t.Helper()
	cmd := exec.Command("ifconfig", name)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), flag)
}

// GetInterfaceOutput returns the full ifconfig output for an interface.
func (h *TestHelper) GetInterfaceOutput(name string) string {
	h.t.Helper()
	cmd := exec.Command("ifconfig", name)
	output, err := cmd.Output()
	if err != nil {
		h.t.Logf("failed to get interface output: %v", err)
		return ""
	}
	return string(output)
}

// AssertInterfaceExists asserts that an interface exists, failing the test if it doesn't.
func (h *TestHelper) AssertInterfaceExists(name string) {
	h.t.Helper()
	if !h.InterfaceExists(name) {
		h.t.Fatalf("interface %s does not exist", name)
	}
}

// AssertInterfaceHasIP asserts that an interface is configured with a specific IP address.
func (h *TestHelper) AssertInterfaceHasIP(name, ip string) {
	h.t.Helper()
	output := h.GetInterfaceOutput(name)
	if !strings.Contains(output, ip) {
		h.t.Errorf("interface %s does not have IP %s\nOutput:\n%s", name, ip, output)
	}
}

// AssertInterfaceIsUp asserts that an interface has the UP flag set.
func (h *TestHelper) AssertInterfaceIsUp(name string) {
	h.t.Helper()
	if !h.InterfaceIsUp(name) {
		h.t.Errorf("interface %s is not UP", name)
	}
}

// AssertInterfaceIsRunning asserts that an interface has the RUNNING flag set.
func (h *TestHelper) AssertInterfaceIsRunning(name string) {
	h.t.Helper()
	if !h.InterfaceIsRunning(name) {
		h.t.Errorf("interface %s is not RUNNING", name)
	}
}

// ListAllUTUNInterfaces returns a list of all UTUN interfaces on the system.
func (h *TestHelper) ListAllUTUNInterfaces() []string {
	h.t.Helper()
	cmd := exec.Command("ifconfig", "-l")
	output, err := cmd.Output()
	if err != nil {
		h.t.Logf("failed to list interfaces: %v", err)
		return nil
	}

	var utunInterfaces []string
	interfaces := strings.Fields(string(output))
	for _, iface := range interfaces {
		if strings.HasPrefix(iface, "utun") {
			utunInterfaces = append(utunInterfaces, iface)
		}
	}

	return utunInterfaces
}

// CountUTUNInterfaces returns the number of UTUN interfaces on the system.
func (h *TestHelper) CountUTUNInterfaces() int {
	h.t.Helper()
	return len(h.ListAllUTUNInterfaces())
}

// LogInterfaceDetails logs detailed information about an interface.
func (h *TestHelper) LogInterfaceDetails(name string) {
	h.t.Helper()
	output := h.GetInterfaceOutput(name)
	h.t.Logf("Interface %s details:\n%s", name, output)
}

// IsRoot returns true if running with root privileges.
func IsRoot() bool {
	return os.Geteuid() == 0
}

// skipIfNotRoot is a standalone helper function for backward compatibility.
func skipIfNotRoot(t *testing.T) {
	t.Helper()
	if !IsRoot() {
		t.Skip("Skipping integration test: requires root privileges (run with sudo)")
	}
}
