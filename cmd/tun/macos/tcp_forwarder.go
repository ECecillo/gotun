//go:build darwin

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// tcpState represents the state of a TCP connection.
type tcpState uint8

const (
	stateClosed tcpState = iota
	stateSynReceived
	stateEstablished
	stateFinWait
)

func (s tcpState) String() string {
	switch s {
	case stateClosed:
		return "CLOSED"
	case stateSynReceived:
		return "SYN_RECEIVED"
	case stateEstablished:
		return "ESTABLISHED"
	case stateFinWait:
		return "FIN_WAIT"
	default:
		return "UNKNOWN"
	}
}

// connectionKey uniquely identifies a TCP connection by 4-tuple.
type connectionKey struct {
	srcIP   [4]byte
	dstIP   [4]byte
	srcPort uint16
	dstPort uint16
}

// tcpConnection represents a single TCP connection being proxied.
type tcpConnection struct {
	key        connectionKey
	serverConn net.Conn
	clientSeq  uint32 // Next expected sequence number from client
	serverSeq  uint32 // Our sequence number to client
	state      tcpState
	lastActive time.Time
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// TCPForwarder handles forwarding TCP packets by proxying connections.
type TCPForwarder struct {
	utunFD      int
	connections map[connectionKey]*tcpConnection
	mu          sync.RWMutex
	idleTimeout time.Duration
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewTCPForwarder creates a new TCPForwarder.
func NewTCPForwarder(utunFD int, idleTimeout time.Duration) *TCPForwarder {
	ctx, cancel := context.WithCancel(context.Background())
	f := &TCPForwarder{
		utunFD:      utunFD,
		connections: make(map[connectionKey]*tcpConnection),
		idleTimeout: idleTimeout,
		ctx:         ctx,
		cancel:      cancel,
	}
	go f.startCleanupLoop()
	return f
}

// Forward processes an incoming TCP packet.
// Implements ProtocolForwarder interface.
func (f *TCPForwarder) Forward(ipPacket []byte) error {
	info, err := parseTCPPacket(ipPacket)
	if err != nil {
		return fmt.Errorf("failed to parse TCP packet: %w", err)
	}

	key := makeConnectionKey(info)

	log.Printf("🔗 TCP: %s:%d -> %s:%d [SEQ=%d ACK=%d] Flags: %s",
		info.SrcIP, info.SrcPort, info.DstIP, info.DstPort,
		info.SeqNum, info.AckNum, flagsToString(info.Flags))

	// Handle RST - always process
	if info.Flags.RST {
		return f.handleRST(key)
	}

	// Handle SYN (connection initiation)
	if info.Flags.SYN && !info.Flags.ACK {
		return f.handleSYN(key, info)
	}

	// Get existing connection
	f.mu.RLock()
	conn, exists := f.connections[key]
	f.mu.RUnlock()

	if !exists {
		// No connection, send RST
		log.Printf("⚠️  TCP: No connection for packet, sending RST")
		return f.sendRST(info)
	}

	conn.mu.Lock()
	conn.lastActive = time.Now()
	conn.mu.Unlock()

	// Handle FIN
	if info.Flags.FIN {
		return f.handleFIN(conn, info)
	}

	// Handle ACK with data or pure ACK
	if info.Flags.ACK {
		return f.handleData(conn, info)
	}

	return nil
}

// Close shuts down the TCP forwarder and all connections.
func (f *TCPForwarder) Close() error {
	f.cancel()
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, conn := range f.connections {
		conn.cancel()
		if conn.serverConn != nil {
			_ = conn.serverConn.Close()
		}
	}
	f.connections = make(map[connectionKey]*tcpConnection)
	log.Printf("🔌 TCP forwarder closed")
	return nil
}

// handleSYN handles a SYN packet to initiate a new connection.
func (f *TCPForwarder) handleSYN(key connectionKey, info *tcpPacketInfo) error {
	log.Printf("📨 TCP SYN: Initiating connection to %s:%d", info.DstIP, info.DstPort)

	// Connect to the real server
	serverAddr := net.JoinHostPort(info.DstIP.String(), fmt.Sprintf("%d", info.DstPort))
	serverConn, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
	if err != nil {
		log.Printf("❌ TCP: Failed to connect to %s: %v", serverAddr, err)
		return f.sendRST(info)
	}

	ctx, cancel := context.WithCancel(f.ctx)
	conn := &tcpConnection{
		key:        key,
		serverConn: serverConn,
		clientSeq:  info.SeqNum + 1, // SYN consumes 1 sequence number
		serverSeq:  1000,            // Initial sequence number
		state:      stateSynReceived,
		lastActive: time.Now(),
		ctx:        ctx,
		cancel:     cancel,
	}

	f.mu.Lock()
	// Close any existing connection with this key
	if existing, ok := f.connections[key]; ok {
		existing.cancel()
		if existing.serverConn != nil {
			_ = existing.serverConn.Close()
		}
	}
	f.connections[key] = conn
	f.mu.Unlock()

	// Send SYN-ACK to client
	if err := f.sendSYNACK(conn, info); err != nil {
		f.removeConnection(key)
		return err
	}

	// Start reading from server
	go f.startServerReader(conn)

	return nil
}

// handleData handles data packets and ACKs.
func (f *TCPForwarder) handleData(conn *tcpConnection, info *tcpPacketInfo) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	// Update state if we were in SYN_RECEIVED and got ACK
	if conn.state == stateSynReceived && info.Flags.ACK {
		conn.state = stateEstablished
		log.Printf("✅ TCP: Connection ESTABLISHED")
	}

	// Check for out-of-order packets (simple approach: drop and let client retransmit)
	if info.SeqNum != conn.clientSeq && len(info.Payload) > 0 {
		log.Printf("⚠️  TCP: Out-of-order packet (expected %d, got %d), dropping",
			conn.clientSeq, info.SeqNum)
		return nil
	}

	// Forward data to server if present
	if len(info.Payload) > 0 {
		log.Printf("📤 TCP: Forwarding %d bytes to server", len(info.Payload))
		_, err := conn.serverConn.Write(info.Payload)
		if err != nil {
			log.Printf("❌ TCP: Failed to write to server: %v", err)
			return f.sendRSTLocked(conn, info)
		}

		// Update expected sequence number
		conn.clientSeq += uint32(len(info.Payload))

		// Send ACK to client
		return f.sendACKLocked(conn, info)
	}

	return nil
}

// handleFIN handles a FIN packet.
func (f *TCPForwarder) handleFIN(conn *tcpConnection, info *tcpPacketInfo) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	log.Printf("📭 TCP FIN: Client closing connection")
	conn.state = stateFinWait
	conn.clientSeq++ // FIN consumes 1 sequence number

	// Close server connection
	if conn.serverConn != nil {
		_ = conn.serverConn.Close()
	}

	// Send FIN-ACK to client
	return f.sendFINACKLocked(conn, info)
}

// handleRST handles a RST packet.
func (f *TCPForwarder) handleRST(key connectionKey) error {
	log.Printf("🔴 TCP RST: Connection reset by client")
	f.removeConnection(key)
	return nil
}

// sendSYNACK sends a SYN-ACK packet to the client.
func (f *TCPForwarder) sendSYNACK(conn *tcpConnection, info *tcpPacketInfo) error {
	packet := buildIPv4TCPPacket(
		info.DstIP, info.SrcIP,
		info.DstPort, info.SrcPort,
		conn.serverSeq, conn.clientSeq,
		tcpFlags{SYN: true, ACK: true},
		65535, nil,
	)
	conn.serverSeq++ // SYN consumes 1 sequence number

	n, err := unix.Write(f.utunFD, packet)
	if err != nil {
		return fmt.Errorf("failed to write SYN-ACK: %w", err)
	}
	log.Printf("📨 TCP: Sent SYN-ACK (%d bytes)", n)
	return nil
}

// sendACKLocked sends an ACK packet to the client (must hold conn.mu).
func (f *TCPForwarder) sendACKLocked(conn *tcpConnection, info *tcpPacketInfo) error {
	packet := buildIPv4TCPPacket(
		info.DstIP, info.SrcIP,
		info.DstPort, info.SrcPort,
		conn.serverSeq, conn.clientSeq,
		tcpFlags{ACK: true},
		65535, nil,
	)

	_, err := unix.Write(f.utunFD, packet)
	if err != nil {
		return fmt.Errorf("failed to write ACK: %w", err)
	}
	return nil
}

// sendFINACKLocked sends a FIN-ACK packet (must hold conn.mu).
func (f *TCPForwarder) sendFINACKLocked(conn *tcpConnection, info *tcpPacketInfo) error {
	packet := buildIPv4TCPPacket(
		info.DstIP, info.SrcIP,
		info.DstPort, info.SrcPort,
		conn.serverSeq, conn.clientSeq,
		tcpFlags{FIN: true, ACK: true},
		65535, nil,
	)
	conn.serverSeq++ // FIN consumes 1 sequence number

	_, err := unix.Write(f.utunFD, packet)
	if err != nil {
		return fmt.Errorf("failed to write FIN-ACK: %w", err)
	}
	log.Printf("📨 TCP: Sent FIN-ACK")
	return nil
}

// sendRST sends a RST packet to reject/reset a connection.
func (f *TCPForwarder) sendRST(info *tcpPacketInfo) error {
	packet := buildIPv4TCPPacket(
		info.DstIP, info.SrcIP,
		info.DstPort, info.SrcPort,
		0, info.SeqNum+1,
		tcpFlags{RST: true, ACK: true},
		0, nil,
	)

	_, err := unix.Write(f.utunFD, packet)
	if err != nil {
		return fmt.Errorf("failed to write RST: %w", err)
	}
	log.Printf("📨 TCP: Sent RST")
	return nil
}

// sendRSTLocked sends RST (must hold conn.mu).
func (f *TCPForwarder) sendRSTLocked(conn *tcpConnection, info *tcpPacketInfo) error {
	conn.state = stateClosed
	return f.sendRST(info)
}

// sendDataToClient sends data from server to client.
func (f *TCPForwarder) sendDataToClient(conn *tcpConnection, data []byte) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.state != stateEstablished {
		return fmt.Errorf("connection not established")
	}

	// Build response packet (swap src/dst from original direction)
	srcIP := net.IPv4(conn.key.dstIP[0], conn.key.dstIP[1], conn.key.dstIP[2], conn.key.dstIP[3])
	dstIP := net.IPv4(conn.key.srcIP[0], conn.key.srcIP[1], conn.key.srcIP[2], conn.key.srcIP[3])

	packet := buildIPv4TCPPacket(
		srcIP, dstIP,
		conn.key.dstPort, conn.key.srcPort,
		conn.serverSeq, conn.clientSeq,
		tcpFlags{ACK: true, PSH: true},
		65535, data,
	)

	conn.serverSeq += uint32(len(data))

	n, err := unix.Write(f.utunFD, packet)
	if err != nil {
		return fmt.Errorf("failed to write data to utun: %w", err)
	}
	log.Printf("📨 TCP: Sent %d bytes to client (%d total)", len(data), n)
	return nil
}

// sendFINToClient sends FIN to client when server closes.
func (f *TCPForwarder) sendFINToClient(conn *tcpConnection) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.state == stateClosed || conn.state == stateFinWait {
		return nil
	}

	conn.state = stateFinWait

	srcIP := net.IPv4(conn.key.dstIP[0], conn.key.dstIP[1], conn.key.dstIP[2], conn.key.dstIP[3])
	dstIP := net.IPv4(conn.key.srcIP[0], conn.key.srcIP[1], conn.key.srcIP[2], conn.key.srcIP[3])

	packet := buildIPv4TCPPacket(
		srcIP, dstIP,
		conn.key.dstPort, conn.key.srcPort,
		conn.serverSeq, conn.clientSeq,
		tcpFlags{FIN: true, ACK: true},
		65535, nil,
	)
	conn.serverSeq++

	_, err := unix.Write(f.utunFD, packet)
	if err != nil {
		return fmt.Errorf("failed to write FIN: %w", err)
	}
	log.Printf("📨 TCP: Sent FIN to client (server closed)")
	return nil
}

// startServerReader reads from the server and sends data to client.
func (f *TCPForwarder) startServerReader(conn *tcpConnection) {
	defer func() {
		_ = f.sendFINToClient(conn)
		f.removeConnection(conn.key)
	}()

	buf := make([]byte, 1500)
	for {
		select {
		case <-conn.ctx.Done():
			return
		default:
		}

		// Set read deadline to allow periodic context checks
		_ = conn.serverConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, err := conn.serverConn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("📥 TCP: Server connection closed: %v", err)
			return
		}

		conn.mu.Lock()
		conn.lastActive = time.Now()
		conn.mu.Unlock()

		if err := f.sendDataToClient(conn, buf[:n]); err != nil {
			log.Printf("❌ TCP: Failed to send data to client: %v", err)
			return
		}
	}
}

// startCleanupLoop periodically removes idle connections.
func (f *TCPForwarder) startCleanupLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			f.cleanupIdleConnections()
		}
	}
}

// cleanupIdleConnections removes connections that have been idle too long.
func (f *TCPForwarder) cleanupIdleConnections() {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	for key, conn := range f.connections {
		conn.mu.Lock()
		idle := now.Sub(conn.lastActive)
		conn.mu.Unlock()

		if idle > f.idleTimeout {
			log.Printf("🧹 TCP: Cleaning up idle connection (idle: %v)", idle)
			conn.cancel()
			if conn.serverConn != nil {
				_ = conn.serverConn.Close()
			}
			delete(f.connections, key)
		}
	}
}

// removeConnection removes a connection from the map.
func (f *TCPForwarder) removeConnection(key connectionKey) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if conn, ok := f.connections[key]; ok {
		conn.cancel()
		if conn.serverConn != nil {
			_ = conn.serverConn.Close()
		}
		delete(f.connections, key)
		log.Printf("🗑️  TCP: Connection removed")
	}
}

// makeConnectionKey creates a connection key from packet info.
func makeConnectionKey(info *tcpPacketInfo) connectionKey {
	key := connectionKey{
		srcPort: info.SrcPort,
		dstPort: info.DstPort,
	}
	copy(key.srcIP[:], info.SrcIP.To4())
	copy(key.dstIP[:], info.DstIP.To4())
	return key
}

// flagsToString returns a string representation of TCP flags.
func flagsToString(f tcpFlags) string {
	var s string
	if f.SYN {
		s += "SYN "
	}
	if f.ACK {
		s += "ACK "
	}
	if f.FIN {
		s += "FIN "
	}
	if f.RST {
		s += "RST "
	}
	if f.PSH {
		s += "PSH "
	}
	if s == "" {
		return "none"
	}
	return s[:len(s)-1]
}
