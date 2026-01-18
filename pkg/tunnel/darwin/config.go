//go:build darwin

package darwin

import (
	"fmt"
	"log"
	"net"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Ifreq represents an interface request structure for ioctl calls.
type Ifreq struct {
	Name [16]byte
	Data [16]byte
}

// NetworkInterfaceConfigurator handles network interface configuration operations.
type NetworkInterfaceConfigurator struct {
	fd int
}

// NewNetworkInterfaceConfigurator creates a new interface configurator with a network manager socket.
func NewNetworkInterfaceConfigurator() (*NetworkInterfaceConfigurator, error) {
	fd, err := OpenKernelNetworkManagerSocket()
	if err != nil {
		return nil, fmt.Errorf("unable to open a socket to our kernel network manager, err: %w", err)
	}
	return &NetworkInterfaceConfigurator{fd: fd}, nil
}

// Close closes the network manager socket.
func (ic *NetworkInterfaceConfigurator) Close() error {
	if err := unix.Close(ic.fd); err != nil {
		return fmt.Errorf("unable to close the socket to our kernel network manager, err: %w", err)
	}
	return nil
}

// Configure is the code equivalent of the following ifconfig command:
//
// ifconfig utun_interface_name 10.0.0.2 10.0.0.1 up
func (ic *NetworkInterfaceConfigurator) Configure(utunInterfaceName string, sourceIP, destinationIP net.IP) error {
	if err := ic.setSourceAddr(utunInterfaceName, sourceIP); err != nil {
		return fmt.Errorf("unable to set utun IPv4 source address interface, err: %w", err)
	}

	if err := ic.setDestinationAddr(utunInterfaceName, destinationIP); err != nil {
		return fmt.Errorf("unable to set utun IPv4 destination address interface, err: %w", err)
	}

	if err := ic.setInterfaceUP(utunInterfaceName); err != nil {
		return fmt.Errorf("unable to set utun interface UP, err: %w", err)
	}

	log.Printf("successfully configured interface %s with source %s and destination %s", utunInterfaceName, sourceIP, destinationIP)
	return nil
}

func (ic *NetworkInterfaceConfigurator) setSourceAddr(utunInterfaceName string, sourceIP net.IP) error {
	return ic.setInterfaceIP(utunInterfaceName, sourceIP, unix.SIOCSIFADDR) //nolint:staticcheck
}

func (ic *NetworkInterfaceConfigurator) setDestinationAddr(utunInterfaceName string, destinationIP net.IP) error {
	return ic.setInterfaceIP(utunInterfaceName, destinationIP, unix.SIOCSIFDSTADDR) //nolint:staticcheck
}

func (ic *NetworkInterfaceConfigurator) setInterfaceIP(utunInterfaceName string, ipAddr net.IP, command uintptr) error {
	var ifreq Ifreq
	copy(ifreq.Name[:], []byte(utunInterfaceName))

	// sin_len is the size of the following sockaddr variable
	// representing a struct used by our kernel controller to set
	// the IP of our UTUN network Interface.
	sinLen := 16
	ifreq.Data[0] = byte(sinLen)
	sinFamily := unix.AF_INET
	ifreq.Data[1] = byte(sinFamily)
	// [2:4] contain the port number, but since we want to have it to 0
	// (interface don't have port number here) we won't set anything.
	copy(ifreq.Data[4:8], ipAddr)
	// [8:16] contain padding, let it to 0.

	// we make the syscall manually since we have no existing method in unix package
	// to do IOCTL call.
	//
	// NOTE: We could use RawSyscall given this StackOverflow explanation:
	// https://stackoverflow.com/questions/16977988/details-of-syscall-rawsyscall-syscall-syscall-in-go
	// it would mean that RawSyscall would be a blocking operation which I don't find really
	// useful for this code.
	if _, _, errno := unix.Syscall(
		unix.SYS_IOCTL, //nolint:staticcheck
		uintptr(ic.fd),
		command,
		uintptr(unsafe.Pointer(&ifreq)),
	); errno != 0 {
		err := unix.ErrnoName(errno)
		return fmt.Errorf("unable to set the ip address of our utun interface, error name: %s (number: %d)", err, errno)
	}

	return nil
}

// setInterfaceUP retrieve current interface flags and append UP and RUNNING flags to
// enable interface.
func (ic *NetworkInterfaceConfigurator) setInterfaceUP(utunInterfaceName string) error {
	var ifr Ifreq
	copy(ifr.Name[:], []byte(utunInterfaceName))

	// same here, no matching unix method for this call.
	if _, _, errno := unix.Syscall(
		unix.SYS_IOCTL, //nolint:staticcheck
		uintptr(ic.fd),
		unix.SIOCGIFFLAGS, //nolint:staticcheck
		uintptr(unsafe.Pointer(&ifr)),
	); errno != 0 {
		return fmt.Errorf("unable to get interface flags with kernel controller, error number: %v", errno)
	}

	// MacOS is in Little Endian so weak bytes are on the left, start left.
	currentFlags := uint16(ifr.Data[0]) | uint16(ifr.Data[1])<<8
	// we will add to existing flag the UP flag and RUNNING which
	// can be useful for MacOS to process our packets.
	newFlags := currentFlags | unix.IFF_UP | unix.IFF_RUNNING

	ifr.Data[0] = byte(newFlags)
	ifr.Data[1] = byte(newFlags >> 8)

	// ask kernel controller to use updated Data as new flags for our
	// UTUN interface
	if _, _, errno := unix.Syscall(
		unix.SYS_IOCTL, //nolint:staticcheck
		uintptr(ic.fd),
		uintptr(unix.SIOCSIFFLAGS), //nolint:staticcheck
		uintptr(unsafe.Pointer(&ifr)),
	); errno != 0 {
		return fmt.Errorf("unable to set interface flags with kernel controller, error: %v", errno)
	}

	return nil
}
