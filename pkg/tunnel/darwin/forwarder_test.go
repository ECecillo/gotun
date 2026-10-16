//go:build darwin

package darwin

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestTCPStateString(t *testing.T) {
	tests := []struct {
		state tcpState
		want  string
	}{
		{stateClosed, "CLOSED"},
		{stateSynReceived, "SYN_RECEIVED"},
		{stateEstablished, "ESTABLISHED"},
		{stateFinWait, "FIN_WAIT"},
		{tcpState(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("tcpState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestFlagsToString(t *testing.T) {
	tests := []struct {
		name  string
		flags tcpFlags
		want  string
	}{
		{"none", tcpFlags{}, "none"},
		{"syn", tcpFlags{SYN: true}, "SYN"},
		{"syn+ack", tcpFlags{SYN: true, ACK: true}, "SYN ACK"},
		{"all", tcpFlags{SYN: true, ACK: true, FIN: true, RST: true, PSH: true}, "SYN ACK FIN RST PSH"},
		{"fin+psh", tcpFlags{FIN: true, PSH: true}, "FIN PSH"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flagsToString(tt.flags); got != tt.want {
				t.Errorf("flagsToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMakeConnectionKey(t *testing.T) {
	info := &tcpPacketInfo{
		SrcIP:   net.IPv4(10, 0, 0, 2),
		DstIP:   net.IPv4(10, 0, 0, 1),
		SrcPort: 51000,
		DstPort: 8080,
	}
	key := makeConnectionKey(info)

	if key.srcIP != [4]byte{10, 0, 0, 2} {
		t.Errorf("srcIP = %v, want [10 0 0 2]", key.srcIP)
	}
	if key.dstIP != [4]byte{10, 0, 0, 1} {
		t.Errorf("dstIP = %v, want [10 0 0 1]", key.dstIP)
	}
	if key.srcPort != 51000 || key.dstPort != 8080 {
		t.Errorf("ports = %d -> %d, want 51000 -> 8080", key.srcPort, key.dstPort)
	}

	// The key must be comparable and stable: identical tuples produce equal keys.
	if key != makeConnectionKey(info) {
		t.Error("makeConnectionKey not deterministic for identical input")
	}

	// A different port yields a different key (usable as a map index).
	other := makeConnectionKey(&tcpPacketInfo{
		SrcIP:   info.SrcIP,
		DstIP:   info.DstIP,
		SrcPort: info.SrcPort + 1,
		DstPort: info.DstPort,
	})
	if key == other {
		t.Error("keys with different src ports compared equal")
	}
}

func TestParseUDPPacketErrors(t *testing.T) {
	// IHL=5 => needs 20+8 = 28 bytes; a shorter packet is rejected.
	short := make([]byte, 20)
	short[0] = 0x45
	if _, err := parseUDPPacket(short); err == nil {
		t.Fatal("parseUDPPacket(20 bytes) expected error, got nil")
	}
}

// TestCleanupIdleConnections exercises the idle-eviction logic by injecting the
// connection's lastActive timestamp (the effective clock input) directly. It
// builds the forwarder struct without NewTCPForwarder so no background cleanup
// goroutine is started, keeping the test deterministic and root-free.
func TestCleanupIdleConnections(t *testing.T) {
	f := &TCPForwarder{
		connections: make(map[connectionKey]*tcpConnection),
		idleTimeout: 30 * time.Second,
	}

	newConn := func(port uint16, lastActive time.Time) connectionKey {
		key := connectionKey{srcPort: port, dstPort: 80}
		ctx, cancel := context.WithCancel(context.Background())
		f.connections[key] = &tcpConnection{
			key:        key,
			state:      stateEstablished,
			lastActive: lastActive,
			ctx:        ctx,
			cancel:     cancel,
		}
		return key
	}

	idleKey := newConn(1111, time.Now().Add(-time.Hour)) // well past idleTimeout
	freshKey := newConn(2222, time.Now())                // active

	f.cleanupIdleConnections()

	if _, ok := f.connections[idleKey]; ok {
		t.Error("idle connection was not evicted")
	}
	if _, ok := f.connections[freshKey]; !ok {
		t.Error("fresh connection was incorrectly evicted")
	}
	if len(f.connections) != 1 {
		t.Errorf("connections remaining = %d, want 1", len(f.connections))
	}
}
