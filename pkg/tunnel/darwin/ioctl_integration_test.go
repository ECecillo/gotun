//go:build darwin && integration

package darwin_test

import (
	"strings"
	"testing"

	"ececillo/gotun/pkg/tunnel/darwin"

	"golang.org/x/sys/unix"
)

func TestRetrieveUTUNControllerID_Integration(t *testing.T) {
	skipIfNotRoot(t)

	// Open kernel control socket
	fd, err := darwin.OpenKernelControlSocket()
	if err != nil {
		t.Fatalf("failed to open kernel control socket: %v", err)
	}
	defer func() {
		_ = unix.Close(fd)
	}()

	// Retrieve UTUN controller info
	info, err := darwin.RetrieveUTUNControllerID(fd)
	if err != nil {
		t.Fatalf("failed to retrieve UTUN controller ID: %v", err)
	}

	// Verify controller ID is non-zero
	if info.Id == 0 {
		t.Error("expected non-zero controller ID")
	}

	// Verify controller name is set correctly
	name := unix.ByteSliceToString(info.Name[:])
	expectedName := "com.apple.net.utun_control"
	if !strings.HasPrefix(name, expectedName) {
		t.Errorf("expected controller name to be %s, got: %s", expectedName, name)
	}

	t.Logf("Successfully retrieved UTUN controller: ID=%d, Name=%s", info.Id, name)
}

func TestConnectToUTUNController_Integration(t *testing.T) {
	skipIfNotRoot(t)

	// Open kernel control socket
	fd, err := darwin.OpenKernelControlSocket()
	if err != nil {
		t.Fatalf("failed to open kernel control socket: %v", err)
	}
	defer func() {
		_ = unix.Close(fd)
	}()

	// Retrieve UTUN controller info
	info, err := darwin.RetrieveUTUNControllerID(fd)
	if err != nil {
		t.Fatalf("failed to retrieve UTUN controller ID: %v", err)
	}

	// Connect to UTUN controller
	if err := darwin.ConnectToUTUNController(fd, info); err != nil {
		t.Fatalf("failed to connect to UTUN controller: %v", err)
	}

	t.Logf("Successfully connected to UTUN controller")
}

func TestGetUTUNInterfaceName_Integration(t *testing.T) {
	skipIfNotRoot(t)

	// Open kernel control socket
	fd, err := darwin.OpenKernelControlSocket()
	if err != nil {
		t.Fatalf("failed to open kernel control socket: %v", err)
	}
	defer func() {
		_ = unix.Close(fd)
	}()

	// Retrieve UTUN controller info
	info, err := darwin.RetrieveUTUNControllerID(fd)
	if err != nil {
		t.Fatalf("failed to retrieve UTUN controller ID: %v", err)
	}

	// Connect to UTUN controller
	if err := darwin.ConnectToUTUNController(fd, info); err != nil {
		t.Fatalf("failed to connect to UTUN controller: %v", err)
	}

	// Get interface name
	name, err := darwin.GetUTUNInterfaceName(fd)
	if err != nil {
		t.Fatalf("failed to get UTUN interface name: %v", err)
	}

	// Verify interface name format
	if !strings.HasPrefix(name, "utun") {
		t.Errorf("expected interface name to start with 'utun', got: %s", name)
	}

	// Verify interface name is not too short or too long
	if len(name) < 5 || len(name) > 16 {
		t.Errorf("unexpected interface name length: %s", name)
	}

	t.Logf("Successfully retrieved interface name: %s", name)
}

func TestUTUNWorkflow_Integration(t *testing.T) {
	skipIfNotRoot(t)

	// Test the complete IOCTL workflow
	t.Log("Step 1: Opening kernel control socket")
	fd, err := darwin.OpenKernelControlSocket()
	if err != nil {
		t.Fatalf("failed to open kernel control socket: %v", err)
	}
	defer func() {
		_ = unix.Close(fd)
	}()
	t.Logf("  ✓ Socket opened: fd=%d", fd)

	t.Log("Step 2: Retrieving UTUN controller ID")
	info, err := darwin.RetrieveUTUNControllerID(fd)
	if err != nil {
		t.Fatalf("failed to retrieve UTUN controller ID: %v", err)
	}
	t.Logf("  ✓ Controller ID retrieved: %d", info.Id)

	t.Log("Step 3: Connecting to UTUN controller")
	if err := darwin.ConnectToUTUNController(fd, info); err != nil {
		t.Fatalf("failed to connect to UTUN controller: %v", err)
	}
	t.Log("  ✓ Connected to UTUN controller")

	t.Log("Step 4: Getting interface name")
	name, err := darwin.GetUTUNInterfaceName(fd)
	if err != nil {
		t.Fatalf("failed to get UTUN interface name: %v", err)
	}
	t.Logf("  ✓ Interface name: %s", name)

	t.Log("Complete IOCTL workflow succeeded!")
}

func TestMultipleUTUNConnections_Integration(t *testing.T) {
	skipIfNotRoot(t)

	// Test that we can create multiple UTUN connections
	numConnections := 5
	fds := make([]int, numConnections)
	names := make([]string, numConnections)

	for i := 0; i < numConnections; i++ {
		// Open socket
		fd, err := darwin.OpenKernelControlSocket()
		if err != nil {
			t.Fatalf("connection %d: failed to open socket: %v", i, err)
		}
		fds[i] = fd

		// Get controller info
		info, err := darwin.RetrieveUTUNControllerID(fd)
		if err != nil {
			_ = unix.Close(fd)
			t.Fatalf("connection %d: failed to get controller ID: %v", i, err)
		}

		// Connect
		if err := darwin.ConnectToUTUNController(fd, info); err != nil {
			_ = unix.Close(fd)
			t.Fatalf("connection %d: failed to connect: %v", i, err)
		}

		// Get name
		name, err := darwin.GetUTUNInterfaceName(fd)
		if err != nil {
			_ = unix.Close(fd)
			t.Fatalf("connection %d: failed to get interface name: %v", i, err)
		}
		names[i] = name

		t.Logf("Connection %d: %s (fd=%d)", i, name, fd)
	}

	// Verify all interfaces have unique names
	seen := make(map[string]bool)
	for _, name := range names {
		if seen[name] {
			t.Errorf("duplicate interface name: %s", name)
		}
		seen[name] = true
	}

	// Clean up
	for i, fd := range fds {
		if err := unix.Close(fd); err != nil {
			t.Errorf("failed to close connection %d: %v", i, err)
		}
	}

	t.Logf("Successfully created and closed %d UTUN connections", numConnections)
}
