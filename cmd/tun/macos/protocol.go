//go:build darwin

package main

// Protocol numbers (RFC 790)
const (
	ProtocolICMP uint8 = 1
	ProtocolTCP  uint8 = 6
	ProtocolUDP  uint8 = 17
)

// utunHeaderLen is the length of the protocol family header prepended by macOS.
const utunHeaderLen = 4

// ProtocolName returns a human-readable name for an IP protocol number.
func ProtocolName(protocol uint8) string {
	switch protocol {
	case ProtocolICMP:
		return "ICMP"
	case ProtocolTCP:
		return "TCP"
	case ProtocolUDP:
		return "UDP"
	default:
		return "Unknown"
	}
}
