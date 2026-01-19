//go:build darwin

package darwin

import (
	"encoding/binary"
	"fmt"
	"net"
)

const (
	tcpHeaderLen    = 20
	ipv4ProtocolTCP = 6
)

// tcpFlags represents TCP control flags.
type tcpFlags struct {
	FIN bool
	SYN bool
	RST bool
	PSH bool
	ACK bool
}

// toByte converts flags to a single byte.
func (f tcpFlags) toByte() byte {
	var b byte
	if f.FIN {
		b |= 0x01
	}
	if f.SYN {
		b |= 0x02
	}
	if f.RST {
		b |= 0x04
	}
	if f.PSH {
		b |= 0x08
	}
	if f.ACK {
		b |= 0x10
	}
	return b
}

// tcpPacketInfo contains extracted information from a TCP packet.
type tcpPacketInfo struct {
	SrcIP   net.IP
	DstIP   net.IP
	SrcPort uint16
	DstPort uint16
	SeqNum  uint32
	AckNum  uint32
	Flags   tcpFlags
	Window  uint16
	Payload []byte
}

// parseTCPPacket extracts TCP packet information from an IP packet.
func parseTCPPacket(ipPacket []byte) (*tcpPacketInfo, error) {
	// Get IP header length (IHL is in the lower 4 bits of byte 0, in 32-bit words)
	ihl := int(ipPacket[0]&0x0F) * 4
	if len(ipPacket) < ihl+tcpHeaderLen {
		return nil, fmt.Errorf("packet too small for TCP header")
	}

	// Extract IPs from IP header
	srcIP := net.IPv4(ipPacket[12], ipPacket[13], ipPacket[14], ipPacket[15])
	dstIP := net.IPv4(ipPacket[16], ipPacket[17], ipPacket[18], ipPacket[19])

	// TCP header starts after IP header
	tcpHeader := ipPacket[ihl:]
	srcPort := binary.BigEndian.Uint16(tcpHeader[0:2])
	dstPort := binary.BigEndian.Uint16(tcpHeader[2:4])
	seqNum := binary.BigEndian.Uint32(tcpHeader[4:8])
	ackNum := binary.BigEndian.Uint32(tcpHeader[8:12])

	// Data offset is in upper 4 bits of byte 12, in 32-bit words
	dataOffset := int(tcpHeader[12]>>4) * 4
	if dataOffset < tcpHeaderLen {
		dataOffset = tcpHeaderLen
	}

	// Flags are in byte 13
	flagsByte := tcpHeader[13]
	flags := tcpFlags{
		FIN: flagsByte&0x01 != 0,
		SYN: flagsByte&0x02 != 0,
		RST: flagsByte&0x04 != 0,
		PSH: flagsByte&0x08 != 0,
		ACK: flagsByte&0x10 != 0,
	}

	window := binary.BigEndian.Uint16(tcpHeader[14:16])

	// Payload starts after TCP header (data offset)
	var payload []byte
	if len(tcpHeader) > dataOffset {
		payload = tcpHeader[dataOffset:]
	}

	return &tcpPacketInfo{
		SrcIP:   srcIP,
		DstIP:   dstIP,
		SrcPort: srcPort,
		DstPort: dstPort,
		SeqNum:  seqNum,
		AckNum:  ackNum,
		Flags:   flags,
		Window:  window,
		Payload: payload,
	}, nil
}

// buildIPv4TCPPacket constructs a complete IPv4+TCP packet with the given parameters.
// Returns the packet ready to be written to the utun interface (with protocol family header).
func buildIPv4TCPPacket(srcIP, dstIP net.IP, srcPort, dstPort uint16, seq, ack uint32, flags tcpFlags, window uint16, payload []byte) []byte {
	tcpLen := tcpHeaderLen + len(payload)
	totalLen := ipv4HeaderLen + tcpLen

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

	// Protocol (TCP = 6)
	ip[9] = ipv4ProtocolTCP

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

	// TCP header starts after IP header
	tcp := ip[ipv4HeaderLen:]

	// Source port
	binary.BigEndian.PutUint16(tcp[0:2], srcPort)

	// Destination port
	binary.BigEndian.PutUint16(tcp[2:4], dstPort)

	// Sequence number
	binary.BigEndian.PutUint32(tcp[4:8], seq)

	// Acknowledgment number
	binary.BigEndian.PutUint32(tcp[8:12], ack)

	// Data offset (5 = 20 bytes, no options) + reserved
	tcp[12] = 5 << 4

	// Flags
	tcp[13] = flags.toByte()

	// Window size
	binary.BigEndian.PutUint16(tcp[14:16], window)

	// Checksum (initially 0, calculated below)
	binary.BigEndian.PutUint16(tcp[16:18], 0)

	// Urgent pointer
	binary.BigEndian.PutUint16(tcp[18:20], 0)

	// Payload
	copy(tcp[tcpHeaderLen:], payload)

	// Calculate TCP checksum with pseudo-header
	tcpChecksum := tcpChecksum(srcIP.To4(), dstIP.To4(), tcp[:tcpLen])
	binary.BigEndian.PutUint16(tcp[16:18], tcpChecksum)

	return packet
}

// tcpChecksum calculates the TCP checksum including the pseudo-header.
func tcpChecksum(srcIP, dstIP net.IP, tcpSegment []byte) uint16 {
	// Pseudo-header: srcIP(4) + dstIP(4) + zero(1) + proto(1) + tcpLen(2) = 12 bytes
	pseudoHeader := make([]byte, 12)
	copy(pseudoHeader[0:4], srcIP)
	copy(pseudoHeader[4:8], dstIP)
	pseudoHeader[8] = 0
	pseudoHeader[9] = ipv4ProtocolTCP
	binary.BigEndian.PutUint16(pseudoHeader[10:12], uint16(len(tcpSegment)))

	// Calculate checksum over pseudo-header + TCP segment
	var sum uint32

	// Sum pseudo-header
	for i := 0; i < len(pseudoHeader); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(pseudoHeader[i : i+2]))
	}

	// Sum TCP segment
	for i := 0; i < len(tcpSegment)-1; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(tcpSegment[i : i+2]))
	}

	// Handle odd length
	if len(tcpSegment)%2 != 0 {
		sum += uint32(tcpSegment[len(tcpSegment)-1]) << 8
	}

	// Fold 32-bit sum to 16 bits
	for sum > 0xFFFF {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}

	return ^uint16(sum)
}
