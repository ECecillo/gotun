//go:build darwin

package darwin

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

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

// RetrieveUTUNControllerID sends an IOCTL call to our kernel controller to retrieve
// Info data containing the UTUN (User Tunnel) Controller ID.
func RetrieveUTUNControllerID(kernelControllerFD int) (unix.CtlInfo, error) {
	var info unix.CtlInfo
	copy(info.Name[:], []byte(ctlName))

	// kernel call to retrieve UTUN Controller ID by filling the Go struct.
	if err := unix.IoctlCtlInfo(kernelControllerFD, &info); err != nil {
		return unix.CtlInfo{}, fmt.Errorf("unable to retrieve ctl controller id from kernel controller, err: %w", err)
	}
	return info, nil
}

// ConnectToUTUNController connects the kernel control socket to the UTUN controller.
func ConnectToUTUNController(kernelControllerFD int, ctlInfo unix.CtlInfo) error {
	const autoAssign = 0
	utunInterfaceChoice := autoAssign
	if err := unix.Connect(kernelControllerFD, &unix.SockaddrCtl{ID: ctlInfo.Id, Unit: uint32(utunInterfaceChoice)}); err != nil {
		return fmt.Errorf("unable to connect to the utun controller, err: %w", err)
	}
	return nil
}

// GetUTUNInterfaceName sends a syscall to UTUN Controller to retrieve the currently
// assigned network interface ID for further processing.
func GetUTUNInterfaceName(utunControllerFD int) (string, error) {
	level := unix.AF_SYS_CONTROL
	socketOpt := unix.AF_INET // 2
	name, err := unix.GetsockoptString(utunControllerFD, level, socketOpt)
	if err != nil {
		return "", fmt.Errorf("unable to get UTUN interface name, err: %w", err)
	}
	return name, nil
}
