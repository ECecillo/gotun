package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

func main() {
	host := flag.String("host", "10.0.0.1", "destination host")
	port := flag.Int("port", 8080, "destination port")
	message := flag.String("msg", "hello from devtool", "message to send")
	timeout := flag.Duration("timeout", 2*time.Second, "response timeout")
	flag.Parse()

	addr := net.JoinHostPort(*host, strconv.Itoa(*port))
	conn, err := net.Dial("udp", addr)
	if err != nil {
		log.Fatalf("Failed to dial UDP %s: %v", addr, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	n, err := conn.Write([]byte(*message))
	if err != nil {
		log.Fatalf("Failed to send UDP packet: %v", err)
	}
	fmt.Printf("Sent %d bytes to %s: %s\n", n, addr, *message)

	// Wait for response
	if err := conn.SetReadDeadline(time.Now().Add(*timeout)); err != nil {
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
