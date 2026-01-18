//go:build darwin

package main

import (
	"encoding/binary"
	"net"
)

const (
	ipv4Version     = 4
	ipv4HeaderLen   = 20
	udpHeaderLen    = 8
	ipv4ProtocolUDP = 17
	defaultTTL      = 64
)

// buildIPv4UDPPacket constructs a complete IPv4+UDP packet with the given parameters.
// Returns the packet ready to be written to the utun interface (with protocol family header).
func buildIPv4UDPPacket(srcIP, dstIP net.IP, srcPort, dstPort uint16, payload []byte) []byte {
	udpLen := udpHeaderLen + len(payload)
	totalLen := ipv4HeaderLen + udpLen

	// 4-byte protocol family header for macOS utun (AF_INET = 2)
	packet := make([]byte, 4+totalLen)
	packet[0] = 0
	packet[1] = 0
	packet[2] = 0
	packet[3] = 2 // AF_INET

	// IP header starts at offset 4
	ip := packet[4:]

	// Version (4) + IHL (5 = 20 bytes)
	ip[0] = (ipv4Version << 4) | 5

	// DSCP + ECN
	ip[1] = 0

	// Total length
	binary.BigEndian.PutUint16(ip[2:4], uint16(totalLen))

	// Identification
	binary.BigEndian.PutUint16(ip[4:6], 0)

	// Flags (Don't Fragment) + Fragment Offset
	binary.BigEndian.PutUint16(ip[6:8], 0x4000)

	// TTL
	ip[8] = defaultTTL

	// Protocol (UDP = 17)
	ip[9] = ipv4ProtocolUDP

	// Header checksum (initially 0, calculated below)
	ip[10] = 0
	ip[11] = 0

	// Source IP
	copy(ip[12:16], srcIP.To4())

	// Destination IP
	copy(ip[16:20], dstIP.To4())

	// Calculate IP header checksum
	checksum := ipChecksum(ip[:ipv4HeaderLen])
	binary.BigEndian.PutUint16(ip[10:12], checksum)

	// UDP header starts after IP header
	udp := ip[ipv4HeaderLen:]

	// Source port
	binary.BigEndian.PutUint16(udp[0:2], srcPort)

	// Destination port
	binary.BigEndian.PutUint16(udp[2:4], dstPort)

	// UDP length
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpLen))

	// UDP checksum (0 is valid for IPv4)
	binary.BigEndian.PutUint16(udp[6:8], 0)

	// Payload
	copy(udp[udpHeaderLen:], payload)

	return packet
}

// ipChecksum calculates the IPv4 header checksum.
func ipChecksum(header []byte) uint16 {
	var sum uint32
	for i := 0; i < len(header); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(header[i : i+2]))
	}
	// Fold 32-bit sum to 16 bits
	for sum > 0xFFFF {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return ^uint16(sum)
}
