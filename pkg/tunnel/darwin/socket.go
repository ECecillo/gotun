package darwin

import (
	"fmt"
	"log"

	"golang.org/x/sys/unix"
)

// OpenKernelControlSocket create and return the id of the file descriptor
// opened to the kernel control.
func OpenKernelControlSocket() (int, error) {
	// fd : file descriptor
	// reminder: when we ask to the kernel to create a socket, a file is created somewhere
	// and this file is assigned a unique identifier returned by this function.
	//
	// Now if we send stuff to this id we will write data into this file or socket here.
	//
	// AF_SYSTEM: general address, saying to the kernel that we want to establish a socket
	// with him not an IPv4 or local file (Unix Socket).
	//
	// SOCK_DGRAM: what type of data we are going to send in this socket, here
	// we choose to send diagrams.
	//
	// AF_SYS_CONTROL: message protocol used for this socket.
	//
	fd, err := unix.Socket(unix.AF_SYSTEM, unix.SOCK_DGRAM, unix.AF_SYS_CONTROL)
	if err != nil {
		return 0, fmt.Errorf("unable to establish kernel constrol socket, err:%w", err)
	}

	log.Println("kernel control socket established!")
	return fd, nil
}

// OpenKernelNetworkManagerSocket create and return the id of the file descriptor
// opened to the network manager that handle all the operation regarding network stack.
func OpenKernelNetworkManagerSocket() (int, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return 0, fmt.Errorf("unable to open a socket to our network manager, err: %w", err)
	}
	log.Println("network manager socket established!")
	return fd, nil
}
