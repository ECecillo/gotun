//go:build darwin

package main

import (
	"fmt"
	"log"
	"net"
	"unsafe"

	"golang.org/x/sys/unix"
)

// openKernelControlSocket create and return the id of the file descriptor
// opened to the kernel control.
func openKernelControlSocket() (int, error) {
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

// openKernelNetworkManagerSocket create and return the id of the file descriptor
// opened to the network manager that handle all the operation regarding network stack.
func openKernelNetworkManagerSocket() (int, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return 0, fmt.Errorf("unable to open a socket to our network manager, err: %w", err)
	}
	log.Println("network manager socket established!")
	return fd, nil
}

// name of the system service that we want to retrieve its ID for MacOS in our case.
const ctlName = "com.apple.net.utun_control"

// -----------------------------------------------------------------------------
// IOCTL Constant Construction
// -----------------------------------------------------------------------------
// The IOCTL command on macOS is a 32-bit integer composed of 4 parts:
// [ Direction (3 bits) | Size (13 bits) | Group (8 bits) | Command (8 bits) ]
// -----------------------------------------------------------------------------

const (
	// 1. Direction: IOC_INOUT
	// Indicates a Read/Write operation.
	// We write the 'Name' to the kernel, and read the 'Id' back.
	// Value: 0xC0000000 (Binary: 1100...)
	iocInOut = 0xC0000000

	// 2. Size: sizeof(struct ctl_info)
	// The kernel needs to know the exact size of the data structure we are passing.
	// We use unsafe.Sizeof to get the size of CtlInfo (100 bytes) and shift it
	// by 16 bits to place it in the correct position for append operation below.
	//
	// Value: 0x00640000 (100 << 16)
	iocSize = uint32(unsafe.Sizeof(unix.CtlInfo{})) << 16

	// 3. Group: 'N'
	// The ASCII character 'N' (0x4E) identifies the "Network Kernel Control" subsystem.
	// Shifted by 8 bits for append operation below.
	//
	// Value: 0x00004E00
	iocGroup = 'N' << 8

	// 4. Command: 3
	// The specific command number for "Get Info" within the 'N' group.
	// Value: 0x00000003
	iocCommand = 3
)

// CTLIOCGINFO is the final constructed IOCTL magic number.
// Equivalent to the C macro: _IOWR('N', 3, struct ctl_info)
// Expected Value: 0xC0644E03
const CTLIOCGINFO = iocInOut | iocSize | iocGroup | iocCommand

// However, this is now hidden by unix package on IoctlCtlInfo call.
// We just put the above definition for explanation.

// retrieveUTUNControllerIDFromKernelController send a IOCTL call to our kernel controller to retrieve
// Info data containing the UTUN (User Tunnel) Controller ID.
func retrieveUTUNControllerIDFromKernelController(kernelControllerFD int) (unix.CtlInfo, error) {
	var info unix.CtlInfo
	copy(info.Name[:], []byte(ctlName))

	// kernel call to retrieve UTUN Controller ID by filling the Go struct.
	if err := unix.IoctlCtlInfo(kernelControllerFD, &info); err != nil {
		return unix.CtlInfo{}, fmt.Errorf("unable to retrieve ctl controller id from kernel controller, err: %w", err)
	}
	return info, nil
}

func connectToUTUNController(kernelControllerFD int, ctlInfo unix.CtlInfo) error {
	const autoAssign = 0
	utunInterfaceChoice := autoAssign
	if err := unix.Connect(kernelControllerFD, &unix.SockaddrCtl{ID: ctlInfo.Id, Unit: uint32(utunInterfaceChoice)}); err != nil {
		return fmt.Errorf("unable to connect to the utun controller, err: %w", err)
	}
	return nil
}

// askForUTUNNetworkInterfaceName send a syscall to UTUN Controller to retrieve the currently
// assigned network interface ID for further processing.
func askForUTUNNetworkInterfaceName(utunControllerFD int) (string, error) {
	level := unix.AF_SYS_CONTROL
	socketOpt := unix.AF_INET // 2
	name, err := unix.GetsockoptString(utunControllerFD, level, socketOpt)
	if err != nil {
		return "", fmt.Errorf("unable to get UTUN interface name, err: %w", err)
	}
	return name, nil
}

// configureUTUNIPInterface is the code equivalent of the following ifconfig
// command:
//
// ifconfig utun_interface_name 10.0.0.2 10.0.0.1 up
func configureUTUNIPInterface(utunInterfaceName string, utunSourceIPAddr net.IP, utunDestinationIPAddr net.IP) error {
	// establish socket connection with kernel controller again
	kernelNetworkManagerFD, err := openKernelNetworkManagerSocket()
	if err != nil {
		return fmt.Errorf("unable to open a socket to our kernel network manager, err: %w", err)
	}
	defer func() {
		if err := unix.Close(kernelNetworkManagerFD); err != nil {
			log.Printf("unable to close the socket to our kernel network manager, err: %v", err)
		}
	}()

	if err := setSourceInterfaceAddr(kernelNetworkManagerFD, utunInterfaceName, utunSourceIPAddr); err != nil {
		return fmt.Errorf("unable to set utun IPv4 source address interface, err: %w", err)
	}

	if err := setDestinationInterfaceAddr(kernelNetworkManagerFD, utunInterfaceName, utunDestinationIPAddr); err != nil {
		return fmt.Errorf("unable to set utun IPv4 destination address interface, err: %w", err)
	}

	if err := setInterfaceUP(kernelNetworkManagerFD, utunInterfaceName); err != nil {
		return fmt.Errorf("unable to set utun interface UP, err: %w", err)
	}
	return nil
}

func setSourceInterfaceAddr(kernelNetworkManagerFD int, utunInterfaceName string, utunSourceIPAddr net.IP) error {
	return setInterfaceIP(kernelNetworkManagerFD, utunInterfaceName, utunSourceIPAddr, unix.SIOCSIFADDR) //nolint:staticcheck
}

func setDestinationInterfaceAddr(kernelNetworkManagerFD int, utunInterfaceName string, utunDestinationIPAddr net.IP) error {
	return setInterfaceIP(kernelNetworkManagerFD, utunInterfaceName, utunDestinationIPAddr, unix.SIOCSIFDSTADDR) //nolint:staticcheck
}

type Ifreq struct {
	Name [16]byte
	Data [16]byte
}

func setInterfaceIP(kernelNetworkManagerFD int, utunInterfaceName string, ipAddr net.IP, command uintptr) error {
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
	if _, _, errno := unix.Syscall(
		unix.SYS_IOCTL, //nolint:staticcheck
		uintptr(kernelNetworkManagerFD),
		command,
		uintptr(unsafe.Pointer(&ifreq)),
	); errno != 0 {
		return fmt.Errorf("unable to set the ip address of our utun interface, error number: %v", errno)
	}

	return nil
}

func setInterfaceUP(kernelNetworkManagerFD int, utunInterfaceName string) error {
	var ifr Ifreq
	copy(ifr.Name[:], []byte(utunInterfaceName))

	if _, _, errno := unix.Syscall(
		unix.SYS_IOCTL, //nolint:staticcheck
		uintptr(kernelNetworkManagerFD),
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
		uintptr(kernelNetworkManagerFD),
		uintptr(unix.SIOCSIFFLAGS), //nolint:staticcheck
		uintptr(unsafe.Pointer(&ifr)),
	); errno != 0 {
		return fmt.Errorf("unable to set interface flags with kernel controller, error: %v", errno)
	}

	return nil
}

func main() {
	kernelControllerFD, err := openKernelControlSocket()
	if err != nil {
		log.Fatalf("unable to open kernel control socket, err: %v", err)
	}

	defer func() {
		err := unix.Close(kernelControllerFD)
		if err != nil {
			log.Printf("unale to close file descriptor, err: %v \n", err)
		}
	}()

	info, err := retrieveUTUNControllerIDFromKernelController(kernelControllerFD)
	if err != nil {
		log.Fatalf("unable to retrieve CTLInfo from kernel controller, err: %v", err)
	}

	log.Println("successfully retrieve CTLInfo from kernel controller")

	if err := connectToUTUNController(kernelControllerFD, info); err != nil {
		log.Fatalf("unable to connect to UTUN Controller, err: %v", err)
	}

	log.Println("successfully connected to a UTUN Controller")
	log.Println("file descriptor now set to UTUN Controller for further syscall")

	utunControllerFD := kernelControllerFD

	utunInterfaceName, err := askForUTUNNetworkInterfaceName(utunControllerFD)
	if err != nil {
		log.Fatalf("unable to retrieve UTUN network interface name, err: %v", err)
	}

	log.Printf("UTUN Interface name is: %s", utunInterfaceName)

	rawSourceAddr := "10.0.0.2"
	sourceAddr := net.ParseIP(rawSourceAddr).To4()
	if sourceAddr == nil {
		log.Fatalf("invalid ipv4 address parsed, addr: %s", rawSourceAddr)
	}

	rawDestinationAddr := "10.0.0.1"
	destinationAddr := net.ParseIP(rawDestinationAddr).To4()
	if sourceAddr == nil {
		log.Fatalf("invalid ipv4 address parsed, addr: %s", rawSourceAddr)
	}

	if err := configureUTUNIPInterface(utunInterfaceName, sourceAddr, destinationAddr); err != nil {
		log.Fatalf("unable to configure utun ip interface, err: %v", err)
	}

	log.Println("successfully enable our UTUN network interface, now be used")
}
