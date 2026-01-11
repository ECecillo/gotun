//go:build darwin

package main

import (
	"log"
	"net"

	"ececillo/gotun/pkg/tunnel"
	"ececillo/gotun/pkg/tunnel/darwin"
)

func main() {
	// Parse IP addresses
	rawSourceAddr := "10.0.0.2"
	sourceAddr := net.ParseIP(rawSourceAddr).To4()
	if sourceAddr == nil {
		log.Fatalf("invalid ipv4 address parsed, addr: %s", rawSourceAddr)
	}

	rawDestinationAddr := "10.0.0.1"
	destinationAddr := net.ParseIP(rawDestinationAddr).To4()
	if destinationAddr == nil {
		log.Fatalf("invalid ipv4 address parsed, addr: %s", rawDestinationAddr)
	}

	// Create tunnel provider
	provider := darwin.NewProvider()
	defer func() {
		if err := provider.Close(); err != nil {
			log.Printf("unable to close tunnel provider, err: %v", err)
		}
	}()

	// Create and configure tunnel
	tun, err := provider.Create(tunnel.TunnelConfig{
		SourceIP:      sourceAddr,
		DestinationIP: destinationAddr,
	})
	if err != nil {
		log.Fatalf("unable to create tunnel, err: %v", err)
	}

	log.Printf("Tunnel created successfully: %s (fd: %d)", tun.Name, tun.FD)
}
