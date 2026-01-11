//go:build darwin && integration

package darwin_test

import (
	"testing"

	"ececillo/gotun/pkg/tunnel/darwin"

	"golang.org/x/sys/unix"
)

func TestOpenKernelControlSocket_Integration(t *testing.T) {
	skipIfNotRoot(t)

	fd, err := darwin.OpenKernelControlSocket()
	if err != nil {
		t.Fatalf("failed to open kernel control socket: %v", err)
	}
	defer func() {
		_ = unix.Close(fd)
	}()

	// Verify file descriptor is valid
	if fd <= 0 {
		t.Errorf("expected positive file descriptor, got: %d", fd)
	}

	// Verify we can get socket info (proves it's a valid socket)
	_, err = unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_TYPE)
	if err != nil {
		t.Errorf("socket appears invalid, failed to get socket type: %v", err)
	}

	t.Logf("Successfully opened kernel control socket: fd=%d", fd)
}

func TestOpenKernelNetworkManagerSocket_Integration(t *testing.T) {
	skipIfNotRoot(t)

	fd, err := darwin.OpenKernelNetworkManagerSocket()
	if err != nil {
		t.Fatalf("failed to open network manager socket: %v", err)
	}
	defer func() {
		_ = unix.Close(fd)
	}()

	// Verify file descriptor is valid
	if fd <= 0 {
		t.Errorf("expected positive file descriptor, got: %d", fd)
	}

	// Verify socket type is SOCK_DGRAM
	sockType, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_TYPE)
	if err != nil {
		t.Fatalf("failed to get socket type: %v", err)
	}

	if sockType != unix.SOCK_DGRAM {
		t.Errorf("expected socket type SOCK_DGRAM (%d), got: %d", unix.SOCK_DGRAM, sockType)
	}

	t.Logf("Successfully opened network manager socket: fd=%d, type=%d", fd, sockType)
}

func TestSocketLifecycle_Integration(t *testing.T) {
	skipIfNotRoot(t)

	// Test opening and closing multiple sockets
	numSockets := 10
	fds := make([]int, numSockets)

	// Open multiple sockets
	for i := 0; i < numSockets; i++ {
		fd, err := darwin.OpenKernelControlSocket()
		if err != nil {
			t.Fatalf("failed to open socket %d: %v", i, err)
		}
		fds[i] = fd
	}

	t.Logf("Successfully opened %d sockets", numSockets)

	// Close all sockets
	for i, fd := range fds {
		if err := unix.Close(fd); err != nil {
			t.Errorf("failed to close socket %d (fd=%d): %v", i, fd, err)
		}
	}

	t.Logf("Successfully closed all sockets")
}
