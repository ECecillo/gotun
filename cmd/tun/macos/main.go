//go:build darwin

package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"

	"ececillo/gotun/pkg/tunnel"
	"ececillo/gotun/pkg/tunnel/darwin"

	"github.com/caarlos0/env/v11"
	"golang.org/x/sys/unix"
)

type config struct {
	SourceIP    string `env:"TUN_SOURCE_IP" envDefault:"10.0.0.2"`
	DestIP      string `env:"TUN_DEST_IP" envDefault:"10.0.0.1"`
	ForwardHost string `env:"TUN_FORWARD_HOST"`
	ForwardPort int    `env:"TUN_FORWARD_PORT" envDefault:"8080"`
}

// forwardUDP extracts the UDP payload from an IP packet and forwards it
// to the specified forwarding address. If forwardAddr is nil, it forwards
// to the original destination from the packet.
func forwardUDP(ipPacket []byte, forwardAddr *net.UDPAddr) error {
	// Get IP header length (IHL is in the lower 4 bits of byte 0, in 32-bit words)
	ihl := int(ipPacket[0]&0x0F) * 4
	if len(ipPacket) < ihl+8 {
		return fmt.Errorf("packet too small for UDP header")
	}

	// UDP header starts after IP header
	udpHeader := ipPacket[ihl:]

	// UDP payload starts after 8-byte UDP header
	udpPayload := udpHeader[8:]

	// Use forward address if provided, otherwise extract from packet
	dstAddr := forwardAddr
	if dstAddr == nil {
		dstIP := net.IPv4(ipPacket[16], ipPacket[17], ipPacket[18], ipPacket[19])
		dstPort := binary.BigEndian.Uint16(udpHeader[2:4])
		dstAddr = &net.UDPAddr{
			IP:   dstIP,
			Port: int(dstPort),
		}
	}

	// Create a UDP connection and send the payload
	conn, err := net.DialUDP("udp", nil, dstAddr)
	if err != nil {
		return fmt.Errorf("failed to dial UDP: %w", err)
	}
	defer conn.Close()

	n, err := conn.Write(udpPayload)
	if err != nil {
		return fmt.Errorf("failed to write UDP payload: %w", err)
	}

	log.Printf("📤 Forwarded %d bytes to %s", n, dstAddr)
	return nil
}

// startReadLoop listens on the utun interface and prints the content of
// the packets. UDP packets are forwarded to forwardAddr if specified.
func startReadLoop(utunFD int, interfaceName string, forwardAddr *net.UDPAddr) {
	log.Printf("🔍 Starting read loop on %s (fd: %d)", interfaceName, utunFD)
	log.Printf("📡 Waiting for packets... (try: ping 10.0.0.1)")

	// MTU (Maximum Transmission Unit) is 1500 bytes.
	// We take a little bit more to handle request with bigger header.
	buffer := make([]byte, 2000)

	for {
		log.Printf("⏳ Waiting for packet...")
		n, err := unix.Read(utunFD, buffer)
		if err != nil {
			log.Printf("❌ Unable to read UTUN interface: %v", err)
			break
		}

		log.Printf("✅ Read %d bytes from interface", n)

		// first 4 bytes are automatically append by MacOS as protocl header.
		// if we don't have more than this then we probably have an empty packet.
		if n <= 4 {
			log.Printf("⚠️  Packet too small (%d bytes), skipping", n)
			continue
		}

		// Log the first 4 bytes (protocol family header)
		protocolFamily := buffer[:4]
		log.Printf("🔧 Protocol family header: %v", protocolFamily)

		ipPacket := buffer[4:n]
		version := ipPacket[0] >> 4

		// Protocol (TCP=6, UDP=17, ICMP=1) on 1 byte
		protocol := ipPacket[9]

		protocolName := "Unknown"
		switch protocol {
		case 1:
			protocolName = "ICMP"
		case 6:
			protocolName = "TCP"
		case 17:
			protocolName = "UDP"
		}

		fmt.Printf("📦 PACKET INTERCEPTED > %d bytes | IPv%d | Protocol: %s (%d)\n",
			len(ipPacket), version, protocolName, protocol)

		// Print source and destination IPs
		if len(ipPacket) >= 20 {
			srcIP := net.IPv4(ipPacket[12], ipPacket[13], ipPacket[14], ipPacket[15])
			dstIP := net.IPv4(ipPacket[16], ipPacket[17], ipPacket[18], ipPacket[19])
			fmt.Printf("   Source: %s -> Destination: %s\n", srcIP, dstIP)
		}

		// Forward UDP packets to their destination
		if protocol == 17 {
			if err := forwardUDP(ipPacket, forwardAddr); err != nil {
				log.Printf("❌ Failed to forward UDP packet: %v", err)
			}
		}
	}
	log.Printf("🛑 Read loop terminated")
}

func main() {
	// Parse environment variables
	cfg, err := env.ParseAs[config]()
	if err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}

	// Build forward address if host is specified
	var forwardAddr *net.UDPAddr
	if cfg.ForwardHost != "" {
		forwardAddr = &net.UDPAddr{
			IP:   net.ParseIP(cfg.ForwardHost),
			Port: cfg.ForwardPort,
		}
		if forwardAddr.IP == nil {
			log.Fatalf("invalid forward host: %s", cfg.ForwardHost)
		}
		log.Printf("UDP forwarding enabled to %s", forwardAddr)
	}

	// Parse IP addresses
	sourceAddr := net.ParseIP(cfg.SourceIP).To4()
	if sourceAddr == nil {
		log.Fatalf("invalid ipv4 address parsed, addr: %s", cfg.SourceIP)
	}

	destinationAddr := net.ParseIP(cfg.DestIP).To4()
	if destinationAddr == nil {
		log.Fatalf("invalid ipv4 address parsed, addr: %s", cfg.DestIP)
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

	go startReadLoop(tun.FD, tun.Name, forwardAddr)
	select {}
}
