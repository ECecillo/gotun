package main

import (
	"fmt"
	"log"
	"net"

	"github.com/caarlos0/env/v11"
)

func main() {
	cfg, err := env.ParseAs[config]()
	if err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}

	port := fmt.Sprintf(":%d", cfg.Port)
	addr, err := net.ResolveUDPAddr("udp", port)
	if err != nil {
		log.Fatalf("Failed to resolve UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP port %s: %v", port, err)
	}
	defer conn.Close()

	fmt.Printf("UDP server listening on %s\n", port)

	buffer := make([]byte, 1024)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		message := string(buffer[:n])
		fmt.Printf("Received %d bytes from %s: %s\n", n, remoteAddr, message)

		// Send response back to the sender
		response := fmt.Sprintf("ACK: %s", message)
		_, err = conn.WriteToUDP([]byte(response), remoteAddr)
		if err != nil {
			log.Printf("Error sending response to %s: %v", remoteAddr, err)
			continue
		}
		fmt.Printf("Sent response to %s: %s\n", remoteAddr, response)
	}
}
