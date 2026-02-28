# Ideas

1. Authentication + key management
    Use per-client certs/keys (mTLS or WireGuard-style keys), short-lived credentials, and key rotation.
2. Access control policies
    Add per-client ACLs (what subnets/ports each client can reach). This is the biggest practical security win.
3. Session management
    Track active sessions, device identity, login history, and support instant revoke/kick.
4. Observability
    Structured logs, per-client metrics (bytes, latency, errors), and health endpoints. You need this before scaling.
5. Reliability hardening
    Reconnect logic, keepalives, idle timeouts, and graceful restart without dropping all clients.
6. DNS + routing controls
    Push DNS settings, split-tunnel/full-tunnel modes, and route conflict detection.
7. Rate limiting + abuse protection
    Per-client bandwidth/connection limits, basic DoS guards, and anomaly alerts.
8. Audit trail
    Record auth events, config changes, policy changes, and admin actions.
