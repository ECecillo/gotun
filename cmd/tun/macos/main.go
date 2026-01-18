//go:build darwin

package main

import (
	"fmt"
	"log"
	"net"

	"ececillo/gotun/pkg/tunnel"
	"ececillo/gotun/pkg/tunnel/darwin"

	"golang.org/x/sys/unix"
)

// readLoop just listen on the utun interface and print on the
// stdout the content of the packages.
func startReadLoop(utunFD int, interfaceName string) {
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
	}
	log.Printf("🛑 Read loop terminated")
}

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

	go startReadLoop(tun.FD, tun.Name)
	select {}
}
