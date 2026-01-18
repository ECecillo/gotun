//go:build darwin

package main

import (
	"fmt"
	"net"
	"time"
)

type config struct {
	SourceIP        string        `env:"TUN_SOURCE_IP" envDefault:"10.0.0.2"`
	DestIP          string        `env:"TUN_DEST_IP" envDefault:"10.0.0.1"`
	ForwardHost     string        `env:"TUN_FORWARD_HOST"`
	ForwardPort     int           `env:"TUN_FORWARD_PORT" envDefault:"8080"`
	ResponseTimeout time.Duration `env:"TUN_RESPONSE_TIMEOUT" envDefault:"5s"`
}

// Validate checks that the configuration values are valid.
func (c *config) Validate() error {
	if c.ParsedSourceIP() == nil {
		return fmt.Errorf("invalid source IP: %s", c.SourceIP)
	}
	if c.ParsedDestIP() == nil {
		return fmt.Errorf("invalid destination IP: %s", c.DestIP)
	}
	if c.ForwardHost != "" && net.ParseIP(c.ForwardHost) == nil {
		return fmt.Errorf("invalid forward host: %s", c.ForwardHost)
	}
	if c.ForwardPort < 0 || c.ForwardPort > 65535 {
		return fmt.Errorf("invalid forward port: %d", c.ForwardPort)
	}
	return nil
}

// ParsedSourceIP returns the source IP as a net.IP (IPv4).
func (c *config) ParsedSourceIP() net.IP {
	return net.ParseIP(c.SourceIP).To4()
}

// ParsedDestIP returns the destination IP as a net.IP (IPv4).
func (c *config) ParsedDestIP() net.IP {
	return net.ParseIP(c.DestIP).To4()
}

// ForwardAddr returns the forward address as a *net.UDPAddr, or nil if not configured.
func (c *config) ForwardAddr() *net.UDPAddr {
	if c.ForwardHost == "" {
		return nil
	}
	return &net.UDPAddr{
		IP:   net.ParseIP(c.ForwardHost),
		Port: c.ForwardPort,
	}
}
