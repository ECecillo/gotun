package tunnel

import "net"

type TunnelProvider interface {
	// Create and configure a Tunnel Network Interface
	Create(config TunnelConfig) (*Tunnel, error)
	// Close cleans up the tunnel interface
	Close() error
}

type TunnelConfig struct {
	SourceIP      net.IP
	DestinationIP net.IP
}

type Tunnel struct {
	Name string
	// FD stands for file descriptor
	FD int
}
