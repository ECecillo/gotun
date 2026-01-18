//go:build darwin

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"ececillo/gotun/pkg/tunnel"
	"ececillo/gotun/pkg/tunnel/darwin"

	"github.com/caarlos0/env/v11"
)

func main() {
	cfg, err := env.ParseAs[config]()
	if err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	if addr := cfg.ForwardAddr(); addr != nil {
		log.Printf("UDP forwarding enabled to %s", addr)
	}

	provider := darwin.NewProvider()
	defer func() {
		if err := provider.Close(); err != nil {
			log.Printf("unable to close tunnel provider: %v", err)
		}
	}()

	tun, err := provider.Create(tunnel.TunnelConfig{
		SourceIP:      cfg.ParsedSourceIP(),
		DestinationIP: cfg.ParsedDestIP(),
	})
	if err != nil {
		log.Fatalf("unable to create tunnel: %v", err)
	}

	log.Printf("Tunnel created successfully: %s (fd: %d)", tun.Name, tun.FD)
	log.Printf("Response timeout: %s", cfg.ResponseTimeout)
	log.Printf("TCP idle timeout: %s", cfg.TCPIdleTimeout)

	// Create forwarders
	udpForwarder := NewUDPForwarder(tun.FD, cfg.ForwardAddr(), cfg.ResponseTimeout)
	tcpForwarder := NewTCPForwarder(tun.FD, cfg.TCPIdleTimeout)

	forwarders := map[uint8]ProtocolForwarder{
		ProtocolUDP: udpForwarder,
		ProtocolTCP: tcpForwarder,
	}

	reader := NewTunnelReader(tun.FD, tun.Name, forwarders)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go reader.Run()

	<-sigChan
	log.Printf("🛑 Shutting down...")
	_ = reader.Close()
}
