// Package tunnel provides a platform-agnostic interface for creating and managing
// network tunnel interfaces (TUN/TAP devices).
//
// This package defines the TunnelProvider interface which abstracts the platform-specific
// details of tunnel creation and configuration. Platform-specific implementations
// (such as Darwin/macOS and Linux) are provided in separate subpackages.
package tunnel
