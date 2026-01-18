package darwin

import (
	"fmt"
	"log"
	"net"
	"unsafe"

	"golang.org/x/sys/unix"
)

// IPRouterConfigurator handles ip router configuration operations.
type IPRouterConfigurator struct {
	fd int
}

func NewIPRouterConfigurator() (*IPRouterConfigurator, error) {
	fd, err := OpenRoutingSocket()
	if err != nil {
		return nil, fmt.Errorf("unable to open a socket to our ip router, err: %w", err)
	}
	return &IPRouterConfigurator{fd: fd}, nil
}

// Close closes the network manager socket.
func (rc *IPRouterConfigurator) Close() error {
	if err := unix.Close(rc.fd); err != nil {
		return fmt.Errorf("unable to close the socket to our ip router, err: %w", err)
	}
	return nil
}

func (rc *IPRouterConfigurator) Configure(utunInterfaceName string, destinationIP net.IP) error {
	if err := rc.addRouteToInterface(destinationIP, utunInterfaceName); err != nil {
		return fmt.Errorf("unable to add route interface, err: %w", err)
	}
	return nil
}

func (rc *IPRouterConfigurator) addRouteToInterface(destinationIP net.IP, ifName string) error {
	// 1. Since unix package does not expose any methods to build
	// a router message we need to retrieve and set ourself all of
	// the field required to add a new IP for the IP Router.

	// A. Retrieve interface id in order to set the IP_LINK of
	// our utun interface
	iface, err := net.InterfaceByName(ifName)
	if err != nil {
		return fmt.Errorf("unable to retrieve utun interface id for name %s, err: %w", ifName, err)
	}

	dstAddr := unix.RawSockaddrInet4{
		Len:    unix.SizeofSockaddrInet4,
		Family: unix.AF_INET,
	}
	copy(dstAddr.Addr[:], destinationIP.To4())

	// B. We are setting the equivalent of the '-interface' argument in
	// route add but instead of interface name we are passing its id:
	//
	// sudo route add -host 10.0.0.1 -interface ifName
	//
	// 'route add' command is resolving the interface id for us.
	linkAddr := unix.SockaddrDatalink{
		Len:    uint8(unix.SizeofSockaddrDatalink),
		Family: unix.AF_LINK,
		Index:  uint16(iface.Index),
	}

	// C. Build the message Header expected on the Socket in order
	// to tell ip router how he is supposed to parse and do with the given
	// Data payload.
	header := unix.RtMsghdr{
		Version: unix.RTM_VERSION,                              // router protocol version.
		Type:    unix.RTM_ADD,                                  // action command (adding a new route).
		Flags:   unix.RTF_UP | unix.RTF_HOST | unix.RTF_STATIC, // enable route, set for a specific host, static routing since we won't change this config a lot.
		Seq:     1,                                             // arbitrary sequence number.
		Pid:     int32(unix.Getpid()),                          // Current process id so router know who is asking route change.
		Addrs:   unix.RTA_DST | unix.RTA_GATEWAY,               // describing to the router data payload. (GATEWAY is for network interface)
	}

	msgSize := unix.SizeofRtMsghdr + unix.SizeofSockaddrInet4 + unix.SizeofSockaddrDatalink
	header.Msglen = uint16(msgSize)
	routerMsg := make([]byte, msgSize)

	// Here we are going to append data according to their unix constant size.
	// Not using append is intended since we could have some padding for some specific systems
	// between data segments.
	copy(routerMsg[0:], (*(*[unix.SizeofRtMsghdr]byte)(unsafe.Pointer(&header)))[:])

	offset := unix.SizeofRtMsghdr
	copy(routerMsg[offset:], (*(*[unix.SizeofSockaddrInet4]byte)(unsafe.Pointer(&dstAddr)))[:])

	offset += unix.SizeofSockaddrInet4
	copy(routerMsg[offset:], (*(*[unix.SizeofSockaddrDatalink]byte)(unsafe.Pointer(&linkAddr)))[:])

	// 2. Writing message to socket
	_, err = unix.Write(rc.fd, routerMsg)
	if err != nil {
		// route already exist
		if err == unix.EEXIST {
			return nil
		}
		return fmt.Errorf("unable to write message buffer to ip router, err: %w", err)
	}

	log.Println("successfully wrote message into ip router socket")
	return nil
}
