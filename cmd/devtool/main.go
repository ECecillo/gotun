package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/caarlos0/env/v11"
)

func main() {
	cfg, err := env.ParseAs[config]()
	if err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := net.Dial("udp", addr)
	if err != nil {
		log.Fatalf("Failed to dial UDP %s: %v", addr, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	n, err := conn.Write([]byte(cfg.Message))
	if err != nil {
		log.Fatalf("Failed to send UDP packet: %v", err)
	}
	fmt.Printf("Sent %d bytes to %s: %s\n", n, addr, cfg.Message)

	// Wait for response
	if err := conn.SetReadDeadline(time.Now().Add(cfg.Timeout)); err != nil {
		log.Fatalf("unable to set connection deadline, err: %v", err)
	}

	buffer := make([]byte, 1024)
	n, err = conn.Read(buffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("No response received (timeout)")
			return
		}
		log.Fatalf("Failed to read response: %v", err)
	}

	fmt.Printf("Received %d bytes: %s\n", n, string(buffer[:n]))
}
