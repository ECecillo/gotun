//go:build darwin

// Package darwin provides the macOS/Darwin-specific implementation of the tunnel.TunnelProvider interface.
//
// This package implements network tunnel creation using macOS's UTUN (User Tunnel) kernel control
// interface. UTUN is the native macOS mechanism for creating virtual network interfaces that can
// be used for VPN, tunneling, and packet manipulation applications.
//
// # References
//
//   - Apple's Network Kernel Extensions: https://developer.apple.com/documentation/networkextension
package darwin
