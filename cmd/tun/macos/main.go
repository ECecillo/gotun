//go:build darwin

package main

import (
	"log"

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

	forwarder := NewUDPForwarder(tun.FD, cfg.ForwardAddr(), cfg.ResponseTimeout)
	reader := NewTunnelReader(tun.FD, tun.Name, forwarder)

	go reader.Run()
	select {}
}
