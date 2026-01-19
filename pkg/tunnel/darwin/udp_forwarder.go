//go:build darwin

package darwin

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"

	"golang.org/x/sys/unix"
)

// udpPacketInfo contains extracted information from a UDP packet.
type udpPacketInfo struct {
	SrcIP   net.IP
	DstIP   net.IP
	SrcPort uint16
	DstPort uint16
	Payload []byte
}

// UDPForwarder handles forwarding UDP packets and writing responses.
type UDPForwarder struct {
	utunFD          int
	forwardAddr     *net.UDPAddr
	responseTimeout time.Duration
}

// NewUDPForwarder creates a new UDPForwarder.
func NewUDPForwarder(utunFD int, forwardAddr *net.UDPAddr, responseTimeout time.Duration) *UDPForwarder {
	return &UDPForwarder{
		utunFD:          utunFD,
		forwardAddr:     forwardAddr,
		responseTimeout: responseTimeout,
	}
}

// Forward forwards a UDP packet and writes any response back to the utun.
func (f *UDPForwarder) Forward(ipPacket []byte) error {
	info, err := parseUDPPacket(ipPacket)
	if err != nil {
		return fmt.Errorf("failed to parse UDP packet: %w", err)
	}

	response, err := f.forwardAndGetResponse(info)
	if err != nil {
		return fmt.Errorf("failed to forward UDP packet: %w", err)
	}

	if response != nil {
		if err := f.writeResponseToTun(info, response); err != nil {
			return fmt.Errorf("failed to write response to utun: %w", err)
		}
	}

	return nil
}

// parseUDPPacket extracts UDP packet information from an IP packet.
func parseUDPPacket(ipPacket []byte) (*udpPacketInfo, error) {
	// Get IP header length (IHL is in the lower 4 bits of byte 0, in 32-bit words)
	ihl := int(ipPacket[0]&0x0F) * 4
	if len(ipPacket) < ihl+8 {
		return nil, fmt.Errorf("packet too small for UDP header")
	}

	// Extract IPs from IP header
	srcIP := net.IPv4(ipPacket[12], ipPacket[13], ipPacket[14], ipPacket[15])
	dstIP := net.IPv4(ipPacket[16], ipPacket[17], ipPacket[18], ipPacket[19])

	// UDP header starts after IP header
	udpHeader := ipPacket[ihl:]
	srcPort := binary.BigEndian.Uint16(udpHeader[0:2])
	dstPort := binary.BigEndian.Uint16(udpHeader[2:4])

	// UDP payload starts after 8-byte UDP header
	payload := udpHeader[8:]

	return &udpPacketInfo{
		SrcIP:   srcIP,
		DstIP:   dstIP,
		SrcPort: srcPort,
		DstPort: dstPort,
		Payload: payload,
	}, nil
}

// forwardAndGetResponse forwards a UDP packet and waits for a response.
func (f *UDPForwarder) forwardAndGetResponse(info *udpPacketInfo) ([]byte, error) {
	// Use forward address if provided, otherwise use packet destination
	dstAddr := f.forwardAddr
	if dstAddr == nil {
		dstAddr = &net.UDPAddr{
			IP:   info.DstIP,
			Port: int(info.DstPort),
		}
	}

	conn, err := net.DialUDP("udp", nil, dstAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial UDP: %w", err)
	}
	defer func() { _ = conn.Close() }()

	n, err := conn.Write(info.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to write UDP payload: %w", err)
	}
	log.Printf("📤 Forwarded %d bytes to %s", n, dstAddr)

	if err := conn.SetReadDeadline(time.Now().Add(f.responseTimeout)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	respBuffer := make([]byte, 2000)
	n, err = conn.Read(respBuffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			log.Printf("⏱️  No response received (timeout)")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("📥 Received response: %d bytes", n)
	return respBuffer[:n], nil
}

// writeResponseToTun constructs a response IP packet and writes it to the utun interface.
func (f *UDPForwarder) writeResponseToTun(originalInfo *udpPacketInfo, responsePayload []byte) error {
	// Build response packet with swapped addresses
	packet := buildIPv4UDPPacket(
		originalInfo.DstIP,   // Response source = original destination
		originalInfo.SrcIP,   // Response destination = original source
		originalInfo.DstPort, // Response source port = original destination port
		originalInfo.SrcPort, // Response destination port = original source port
		responsePayload,
	)

	n, err := unix.Write(f.utunFD, packet)
	if err != nil {
		return fmt.Errorf("failed to write to utun: %w", err)
	}

	log.Printf("📨 Wrote %d bytes response to utun (payload: %d bytes)", n, len(responsePayload))
	return nil
}

// Close implements ProtocolForwarder interface (no-op for UDP).
func (f *UDPForwarder) Close() error {
	return nil
}

var _ ProtocolForwarder = new(UDPForwarder)
