//go:build darwin

package main

import (
	"fmt"
	"log"
	"net"

	"golang.org/x/sys/unix"
)

// ProtocolForwarder is an interface for forwarding IP packets by protocol.
type ProtocolForwarder interface {
	Forward(ipPacket []byte) error
	Close() error
}

// TunnelReader reads packets from a utun interface and processes them.
type TunnelReader struct {
	fd         int
	name       string
	forwarders map[uint8]ProtocolForwarder
}

// NewTunnelReader creates a new TunnelReader.
func NewTunnelReader(fd int, name string, forwarders map[uint8]ProtocolForwarder) *TunnelReader {
	return &TunnelReader{
		fd:         fd,
		name:       name,
		forwarders: forwarders,
	}
}

// Run starts the read loop, blocking until an error occurs.
func (r *TunnelReader) Run() {
	log.Printf("🔍 Starting read loop on %s (fd: %d)", r.name, r.fd)
	log.Printf("📡 Waiting for packets... (try: ping 10.0.0.1)")

	// MTU is 1500 bytes, buffer slightly larger for headers
	buffer := make([]byte, 2000)

	for {
		log.Printf("⏳ Waiting for packet...")

		ipPacket, err := r.readPacket(buffer)
		if err != nil {
			log.Printf("❌ %v", err)
			break
		}

		if ipPacket == nil {
			continue
		}

		r.handlePacket(ipPacket)
	}

	log.Printf("🛑 Read loop terminated")
}

// readPacket reads a single packet from the utun interface.
// Returns the IP packet (without utun header) or nil if the packet should be skipped.
func (r *TunnelReader) readPacket(buffer []byte) ([]byte, error) {
	n, err := unix.Read(r.fd, buffer)
	if err != nil {
		return nil, fmt.Errorf("unable to read UTUN interface: %w", err)
	}

	log.Printf("✅ Read %d bytes from interface", n)

	// First 4 bytes are the protocol family header added by macOS
	if n <= utunHeaderLen {
		log.Printf("⚠️  Packet too small (%d bytes), skipping", n)
		return nil, nil
	}

	protocolFamily := buffer[:utunHeaderLen]
	log.Printf("🔧 Protocol family header: %v", protocolFamily)

	return buffer[utunHeaderLen:n], nil
}

// handlePacket processes a single IP packet.
func (r *TunnelReader) handlePacket(ipPacket []byte) {
	version := ipPacket[0] >> 4
	protocol := ipPacket[9]

	fmt.Printf("📦 PACKET INTERCEPTED > %d bytes | IPv%d | Protocol: %s (%d)\n",
		len(ipPacket), version, ProtocolName(protocol), protocol)

	// Print source and destination IPs
	if len(ipPacket) >= 20 {
		srcIP := net.IPv4(ipPacket[12], ipPacket[13], ipPacket[14], ipPacket[15])
		dstIP := net.IPv4(ipPacket[16], ipPacket[17], ipPacket[18], ipPacket[19])
		fmt.Printf("   Source: %s -> Destination: %s\n", srcIP, dstIP)
	}

	// Dispatch to appropriate forwarder
	if forwarder, ok := r.forwarders[protocol]; ok {
		if err := forwarder.Forward(ipPacket); err != nil {
			log.Printf("❌ %v", err)
		}
	}
}

// Close closes all forwarders.
func (r *TunnelReader) Close() error {
	for _, f := range r.forwarders {
		if err := f.Close(); err != nil {
			log.Printf("⚠️  Error closing forwarder: %v", err)
		}
	}
	return nil
}
