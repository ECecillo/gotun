//go:build darwin

package darwin

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
)

// stripUtunHeader removes the 4-byte macOS protocol-family header that
// buildIPv4TCPPacket / buildIPv4UDPPacket prepend, yielding the raw IP packet
// that parseTCPPacket / parseUDPPacket expect.
func stripUtunHeader(packet []byte) []byte {
	return packet[utunHeaderLen:]
}

func TestTCPFlagsToByte(t *testing.T) {
	tests := []struct {
		name  string
		flags tcpFlags
		want  byte
	}{
		{"none", tcpFlags{}, 0x00},
		{"fin", tcpFlags{FIN: true}, 0x01},
		{"syn", tcpFlags{SYN: true}, 0x02},
		{"rst", tcpFlags{RST: true}, 0x04},
		{"psh", tcpFlags{PSH: true}, 0x08},
		{"ack", tcpFlags{ACK: true}, 0x10},
		{"syn+ack", tcpFlags{SYN: true, ACK: true}, 0x12},
		{"fin+ack", tcpFlags{FIN: true, ACK: true}, 0x11},
		{"all", tcpFlags{FIN: true, SYN: true, RST: true, PSH: true, ACK: true}, 0x1F},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.flags.toByte(); got != tt.want {
				t.Fatalf("toByte() = %#02x, want %#02x", got, tt.want)
			}
		})
	}
}

func TestProtocolName(t *testing.T) {
	tests := []struct {
		proto uint8
		want  string
	}{
		{ProtocolICMP, "ICMP"},
		{ProtocolTCP, "TCP"},
		{ProtocolUDP, "UDP"},
		{0, "Unknown"},
		{99, "Unknown"},
	}
	for _, tt := range tests {
		if got := ProtocolName(tt.proto); got != tt.want {
			t.Errorf("ProtocolName(%d) = %q, want %q", tt.proto, got, tt.want)
		}
	}
}

func TestBuildParseTCPRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		flags   tcpFlags
		window  uint16
		payload []byte
	}{
		{"syn no payload", tcpFlags{SYN: true}, 65535, nil},
		{"syn+ack", tcpFlags{SYN: true, ACK: true}, 4096, nil},
		{"data push+ack", tcpFlags{PSH: true, ACK: true}, 1024, []byte("hello world")},
		{"fin+ack", tcpFlags{FIN: true, ACK: true}, 512, nil},
		{"odd length payload", tcpFlags{ACK: true}, 256, []byte("odd")},
	}

	srcIP := net.IPv4(10, 0, 0, 2)
	dstIP := net.IPv4(10, 0, 0, 1)
	const (
		srcPort uint16 = 51000
		dstPort uint16 = 8080
		seq     uint32 = 0x11223344
		ack     uint32 = 0x55667788
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet := buildIPv4TCPPacket(srcIP, dstIP, srcPort, dstPort, seq, ack, tt.flags, tt.window, tt.payload)

			// The utun protocol-family header must be AF_INET (2).
			if got := packet[:utunHeaderLen]; !bytes.Equal(got, []byte{0, 0, 0, 2}) {
				t.Fatalf("utun header = %v, want [0 0 0 2]", got)
			}

			ipPacket := stripUtunHeader(packet)

			// IP header checksum must be valid (RFC 1071: recomputing over the
			// header with the checksum in place yields 0).
			if sum := ipChecksum(ipPacket[:ipv4HeaderLen]); sum != 0 {
				t.Fatalf("IP checksum not valid, recompute = %#04x, want 0", sum)
			}

			info, err := parseTCPPacket(ipPacket)
			if err != nil {
				t.Fatalf("parseTCPPacket() error = %v", err)
			}

			if !info.SrcIP.Equal(srcIP) {
				t.Errorf("SrcIP = %v, want %v", info.SrcIP, srcIP)
			}
			if !info.DstIP.Equal(dstIP) {
				t.Errorf("DstIP = %v, want %v", info.DstIP, dstIP)
			}
			if info.SrcPort != srcPort {
				t.Errorf("SrcPort = %d, want %d", info.SrcPort, srcPort)
			}
			if info.DstPort != dstPort {
				t.Errorf("DstPort = %d, want %d", info.DstPort, dstPort)
			}
			if info.SeqNum != seq {
				t.Errorf("SeqNum = %d, want %d", info.SeqNum, seq)
			}
			if info.AckNum != ack {
				t.Errorf("AckNum = %d, want %d", info.AckNum, ack)
			}
			if info.Flags != tt.flags {
				t.Errorf("Flags = %+v, want %+v", info.Flags, tt.flags)
			}
			if info.Window != tt.window {
				t.Errorf("Window = %d, want %d", info.Window, tt.window)
			}
			if !bytes.Equal(info.Payload, tt.payload) {
				t.Errorf("Payload = %q, want %q", info.Payload, tt.payload)
			}

			// The embedded TCP checksum must verify (RFC 1071 zero property).
			tcpSegment := ipPacket[ipv4HeaderLen:]
			if sum := tcpChecksum(srcIP.To4(), dstIP.To4(), tcpSegment); sum != 0 {
				t.Fatalf("TCP checksum not valid, recompute = %#04x, want 0", sum)
			}
		})
	}
}

func TestParseTCPPacketErrors(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		// IHL=5 => 20-byte IP header; needs 20+20=40 bytes total.
		{"one byte", []byte{0x45}},
		{"ip header only", make20ByteIPHeader()},
		// A UDP-sized packet (IP header + 8-byte UDP header = 28) is still too
		// short to hold a TCP header, so parseTCPPacket rejects it.
		{"udp sized too short", make([]byte, 28)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure IHL nibble is set so the length guard is what triggers.
			if len(tt.input) > 0 {
				tt.input[0] = 0x45
			}
			if _, err := parseTCPPacket(tt.input); err == nil {
				t.Fatalf("parseTCPPacket(%d bytes) expected error, got nil", len(tt.input))
			}
		})
	}
}

func make20ByteIPHeader() []byte {
	b := make([]byte, 20)
	b[0] = 0x45
	return b
}

func TestIPChecksum(t *testing.T) {
	// Known vector (classic RFC 1071 / Wikipedia example). The checksum field
	// (bytes 10-11) is zeroed for the computation.
	header := []byte{
		0x45, 0x00, 0x00, 0x73, 0x00, 0x00, 0x40, 0x00,
		0x40, 0x11, 0x00, 0x00, 0xc0, 0xa8, 0x00, 0x01,
		0xc0, 0xa8, 0x00, 0xc7,
	}
	if got := ipChecksum(header); got != 0xb861 {
		t.Fatalf("ipChecksum known vector = %#04x, want 0xb861", got)
	}

	// RFC 1071 property: with the checksum written in place, recomputing yields 0.
	binary.BigEndian.PutUint16(header[10:12], 0xb861)
	if got := ipChecksum(header); got != 0 {
		t.Fatalf("ipChecksum with checksum in place = %#04x, want 0", got)
	}
}

func TestTCPChecksum(t *testing.T) {
	// Hand-verifiable known vector: zero IPs and a 20-byte all-zero segment.
	// Pseudo-header contributes proto(6) + tcpLen(20=0x14) = 0x001A, segment
	// adds 0, so the checksum is ^0x001A = 0xFFE5.
	zeroIP := net.IPv4(0, 0, 0, 0).To4()
	segment := make([]byte, tcpHeaderLen)
	if got := tcpChecksum(zeroIP, zeroIP, segment); got != 0xffe5 {
		t.Fatalf("tcpChecksum known vector = %#04x, want 0xffe5", got)
	}

	// RFC 1071 property: writing the checksum back into the segment and
	// recomputing yields 0.
	srcIP := net.IPv4(192, 168, 0, 1).To4()
	dstIP := net.IPv4(192, 168, 0, 199).To4()
	seg := make([]byte, tcpHeaderLen+5)
	copy(seg[tcpHeaderLen:], []byte("abcde"))
	binary.BigEndian.PutUint16(seg[0:2], 1234) // src port
	binary.BigEndian.PutUint16(seg[2:4], 80)   // dst port
	sum := tcpChecksum(srcIP, dstIP, seg)
	binary.BigEndian.PutUint16(seg[16:18], sum) // checksum field
	if got := tcpChecksum(srcIP, dstIP, seg); got != 0 {
		t.Fatalf("tcpChecksum with checksum in place = %#04x, want 0", got)
	}
}

func TestBuildIPv4UDPPacket(t *testing.T) {
	srcIP := net.IPv4(10, 0, 0, 1)
	dstIP := net.IPv4(10, 0, 0, 2)
	const (
		srcPort uint16 = 8080
		dstPort uint16 = 51000
	)
	payload := []byte("hello from devtool")

	packet := buildIPv4UDPPacket(srcIP, dstIP, srcPort, dstPort, payload)

	// utun protocol-family header.
	if got := packet[:utunHeaderLen]; !bytes.Equal(got, []byte{0, 0, 0, 2}) {
		t.Fatalf("utun header = %v, want [0 0 0 2]", got)
	}

	ip := packet[utunHeaderLen:]
	wantTotalLen := ipv4HeaderLen + udpHeaderLen + len(payload)
	if len(ip) != wantTotalLen {
		t.Fatalf("IP packet length = %d, want %d", len(ip), wantTotalLen)
	}

	// Version/IHL.
	if ip[0] != (ipv4Version<<4)|5 {
		t.Errorf("version/IHL byte = %#02x, want %#02x", ip[0], (ipv4Version<<4)|5)
	}
	// Total length field.
	if got := binary.BigEndian.Uint16(ip[2:4]); int(got) != wantTotalLen {
		t.Errorf("IP total length = %d, want %d", got, wantTotalLen)
	}
	// TTL.
	if ip[8] != defaultTTL {
		t.Errorf("TTL = %d, want %d", ip[8], defaultTTL)
	}
	// Protocol = UDP.
	if ip[9] != ipv4ProtocolUDP {
		t.Errorf("protocol = %d, want %d", ip[9], ipv4ProtocolUDP)
	}
	// IP checksum valid (RFC 1071 property).
	if sum := ipChecksum(ip[:ipv4HeaderLen]); sum != 0 {
		t.Errorf("IP checksum not valid, recompute = %#04x, want 0", sum)
	}

	udp := ip[ipv4HeaderLen:]
	if got := binary.BigEndian.Uint16(udp[0:2]); got != srcPort {
		t.Errorf("UDP src port = %d, want %d", got, srcPort)
	}
	if got := binary.BigEndian.Uint16(udp[2:4]); got != dstPort {
		t.Errorf("UDP dst port = %d, want %d", got, dstPort)
	}
	wantUDPLen := udpHeaderLen + len(payload)
	if got := binary.BigEndian.Uint16(udp[4:6]); int(got) != wantUDPLen {
		t.Errorf("UDP length = %d, want %d", got, wantUDPLen)
	}
	if !bytes.Equal(udp[udpHeaderLen:], payload) {
		t.Errorf("UDP payload = %q, want %q", udp[udpHeaderLen:], payload)
	}

	// Round-trip through parseUDPPacket.
	info, err := parseUDPPacket(ip)
	if err != nil {
		t.Fatalf("parseUDPPacket() error = %v", err)
	}
	if !info.SrcIP.Equal(srcIP) || !info.DstIP.Equal(dstIP) {
		t.Errorf("parsed IPs = %v -> %v, want %v -> %v", info.SrcIP, info.DstIP, srcIP, dstIP)
	}
	if info.SrcPort != srcPort || info.DstPort != dstPort {
		t.Errorf("parsed ports = %d -> %d, want %d -> %d", info.SrcPort, info.DstPort, srcPort, dstPort)
	}
	if !bytes.Equal(info.Payload, payload) {
		t.Errorf("parsed payload = %q, want %q", info.Payload, payload)
	}
}
