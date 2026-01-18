//go:build darwin

package darwin

import (
	"fmt"
	"log"

	"ececillo/gotun/pkg/tunnel"

	"golang.org/x/sys/unix"
)

// Provider is the Darwin-specific implementation of the TunnelProvider interface.
type Provider struct {
	fd int
}

// NewProvider creates a new Darwin tunnel provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Create creates and configures a UTUN network interface.
func (p *Provider) Create(config tunnel.TunnelConfig) (*tunnel.Tunnel, error) {
	// 1. Open kernel control socket in order to send IOCTL call
	fd, err := OpenKernelControlSocket()
	if err != nil {
		return nil, fmt.Errorf("unable to open kernel control socket, err: %w", err)
	}
	// Set the provider file descriptor for further processing.
	p.fd = fd

	// 2. Retrieve UTUN controller info
	info, err := RetrieveUTUNControllerID(fd)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("unable to retrieve CTLInfo from kernel controller, err: %w", err)
	}
	log.Println("successfully retrieved CTLInfo from kernel controller")

	// 3. Connect to UTUN controller
	if err := ConnectToUTUNController(fd, info); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("unable to connect to UTUN Controller, err: %w", err)
	}
	log.Println("successfully connected to UTUN Controller")
	log.Println("file descriptor now set to UTUN Controller for further syscall")

	// 4. Get the interface name in order to configure its IP interface
	interfaceName, err := GetUTUNInterfaceName(fd)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("unable to retrieve UTUN network interface name, err: %w", err)
	}
	log.Printf("UTUN Interface name is: %s", interfaceName)

	// 5. Configure the interface with IP addresses
	networkInterfaceConfigurator, err := NewNetworkInterfaceConfigurator()
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("unable to create interface configurator, err: %w", err)
	}
	defer func() {
		_ = networkInterfaceConfigurator.Close()
	}()

	if err := networkInterfaceConfigurator.Configure(interfaceName, config.SourceIP, config.DestinationIP); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("unable to configure utun ip interface, err: %w", err)
	}

	log.Println("successfully enabled UTUN network interface")

	return &tunnel.Tunnel{
		Name: interfaceName,
		FD:   fd,
	}, nil
}

// Close closes the tunnel and releases resources.
func (p *Provider) Close() error {
	if p.fd > 0 {
		if err := unix.Close(p.fd); err != nil {
			return fmt.Errorf("unable to close file descriptor, err: %w", err)
		}
		p.fd = 0
	}
	return nil
}
