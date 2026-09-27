# Torana Enterprise Control Plane Service

The **Torana Enterprise Control Plane** (`torana-controlplane`) is a unified, production-grade management service and dashboard built with **Go** and **React**. It replaces the test-only mock platform with a secure, highly scalable control plane that orchestrates data plane gateway instances (`gateway-data`), signs and broadcasts dynamic configuration snapshots, monitors fleet health, and pushes sub-second credential revocations.

---

## Architecture Overview

```
                        ┌────────────────────────────────────────────────────────┐
                        │              Torana Control Plane Service              │
                        │                                                        │
┌─────────────────┐     │  ┌───────────────┐  REST & SSE  ┌───────────────────┐  │
│  Browser / UI   │◄────┼─►│ React Console │◄────────────►│  HTTP REST Server │  │
│ (Admin Operator)│     │  │  (Vite + CSS) │              │    (:8080)        │  │
└─────────────────┘     │  └───────────────┘              └────────┬──────────┘  │
                        │                                          │             │
                        │  ┌───────────────┐              ┌────────▼──────────┐  │
                        │  │  State Store  │◄─────────────┤ Fleet Manager &   │  │
                        │  │ (Ed25519 Sig) │              │ Security RBAC     │  │
                        │  └───────┬───────┘              └────────┬──────────┘  │
                        │          │                               │             │
                        │          └──────────────┬────────────────┘             │
                        │                         │                              │
                        │                 ┌───────▼────────┐                     │
                        │                 │  gRPC Server   │                     │
                        │                 │    (:9090)     │                     │
                        └─────────────────┴───────┬────────┴─────────────────────┘
                                                  │
                                                  │ gRPC Bidirectional Stream
                                                  │ (Signed Snapshots, ACKs/NACKs,
                                                  │  Heartbeats, Revocations)
                                                  │
                                    ┌─────────────┴─────────────┐
                                    ▼                           ▼
                        ┌───────────────────────┐   ┌───────────────────────┐
                        │  gateway-data Pod 1   │   │  gateway-data Pod 2   │
                        │ (Stateless Data Plane)│   │ (Stateless Data Plane)│
                        └───────────────────────┘   └───────────────────────┘
```

---

## 1. System Requirements & Specifications

### 1.1 Security & Threat Model
- **Trust Boundary & Centralized Truth**: The control plane is the cryptographic root of trust. Data plane instances execute zero persistent local storage and rely entirely on signed snapshots.
- **Fail-Closed Execution**: If an incoming snapshot fails signature verification or CEL compilation, the data plane discards it, emits a `Nack`, and maintains its active state without compromising traffic.
- **Cryptographic Provenance**: Every configuration snapshot is signed using an **Ed25519** private key managed by the control plane. Gateways verify `ed25519.Verify(pubKey, "config_version:<ver>", signature)` before applying.
- **Zero Customer Payload Exfiltration**: Data plane nodes never stream raw prompts, model outputs, tool parameters, or completions to the control plane. The control stream only ingests anonymized telemetry: token counts, status codes, route IDs, and duration.
- **Secret Reference Isolation**: Configuration snapshots reference secrets indirectly (`SecretRef`), e.g., `vault://production/openai-key`. Raw secrets never travel across the control plane stream.

### 1.2 Authentication & Authorization (AuthN / AuthZ)
- **Node Enrollment**:
  - Gateways must present a valid `enroll_token` during `Enroll(EnrollRequest)` RPC.
  - Rejection with `Accepted: false` if token is invalid or expired.
- **Operator Authentication**:
  - Management HTTP APIs and UI support Bearer tokens, `X-API-Key`, or session cookies.
  - Role-Based Access Control (RBAC):
    - `admin`: Full access — edit config, publish snapshots, perform rollbacks, issue credential revocations.
    - `operator`: Can view fleet, inspect telemetry, issue revocations.
    - `viewer`: Read-only access to metrics, nodes, and audit logs.
- **Immediate Revocation**:
  - Operators can revoke compromised API keys or bearer tokens in the UI.
  - Broadcasted via `ControlMessage.Revocation` to all connected gateways within sub-second latency.

### 1.3 Data Plane Communication Protocol
- **gRPC Service**: `controlplane.v1.ControlPlaneService`
  - `rpc Enroll(EnrollRequest) returns (EnrollResponse)`
  - `rpc Stream(stream NodeMessage) returns (stream ControlMessage)`
- **Node-to-Platform Messages (`NodeMessage`)**:
  - `Hello`: Gateway declares supported protocols (HTTP, gRPC, WebSocket, MCP, A2A) and runtime version on connect.
  - `Heartbeat`: Real-time health signals (active connections, active streams, memory heap bytes, CPU permille, current config version).
  - `Ack`: Confirms atomic, zero-downtime application of a published version.
  - `Nack`: Alerts the control plane of rejection (e.g. signature error, CEL compilation failure) with reason and error code.
  - `UsageReport`: Bounded batches of prompt/completion tokens and request counts per tenant/route/model.
  - `PluginStatus`: Status and latency of attached dynamic plugins.
- **Platform-to-Node Messages (`ControlMessage`)**:
  - `Snapshot`: Complete, self-contained signed routing tables, CEL policies, upstream clusters, authn providers, and token budgets.
  - `Delta`: Incremental additions and deletions.
  - `Revocation`: List of revoked tokens and API keys.
  - `Command`: Operational commands (`DRAIN`, `RELOAD`, `PROBE`).

### 1.4 State Management & Versioning
- Monotonically increasing `config_version`.
- Atomic snapshot compilation: `sync/atomic.Pointer` guarantees no partial reads or lock contention.
- Version history preserves snapshots for instant one-click rollback.
- Persistence to disk JSON preserves configuration state across restarts.

---

## 2. Directory Structure

```
controlplane/
├── cmd/
│   └── server/
│       └── main.go              # Control plane service entrypoint (gRPC + REST + UI)
├── pkg/
│   ├── auth/                    # RBAC, token verification, audit logger
│   ├── crypto/                  # Ed25519 keypair generator & snapshot signer
│   ├── fleet/                   # Gateway registry, stream channels, telemetry aggregator
│   ├── grpc/                    # ControlPlaneServiceServer gRPC implementation
│   ├── rest/                    # HTTP REST API, SSE event stream, static UI server
│   └── state/                   # Declarative schema, atomic versioning, rollback, persistence
├── ui/                          # React + Vite frontend
│   ├── src/
│   │   ├── components/
│   │   │   ├── Header.jsx       # PubKey, active version, live status
│   │   │   ├── FleetView.jsx    # Node metrics, health badges, ACK/NACK state
│   │   │   ├── ConfigStudio.jsx # Interactive editor for Routes, Upstreams, Policies
│   │   │   ├── RolloutPublisher.jsx # Ed25519 signer & real-time rollout progress
│   │   │   ├── RevocationManager.jsx# Emergency blocklist broadcaster
│   │   │   ├── TelemetryView.jsx# Token analytics per tenant & model
│   │   │   └── AuditLogView.jsx # Security event trail
│   │   ├── App.jsx              # Tab navigation & global state
│   │   ├── index.css            # Dark mode glassmorphism design system
│   │   └── api.js               # REST & SSE client
│   ├── embed.go                 # Embeds compiled React UI into Go binary
│   ├── package.json
│   └── vite.config.js
└── README.md
```

---

## 3. Quickstart & Usage

### 3.1 Build Everything
```bash
# Build React UI and Go binary into bin/torana-controlplane
make controlplane
```

### 3.2 Run the Control Plane Service
```bash
# Start gRPC on :9090 and Web Console on :8080
make run-controlplane
```
Access the React web console at: **http://localhost:8080**

### 3.3 CLI Flags
```bash
./bin/torana-controlplane \
  -grpc-port :9090 \
  -http-port :8080 \
  -storage-file controlplane/data/config-state.json \
  -key-file controlplane/data/ed25519.key \
  -admin-token torana-admin-secret-key \
  -enroll-token torana-enroll-dev-secret
```

### 3.4 Connecting a Gateway Data Plane Node
Run `gateway-data` pointing to the control plane:
```bash
./bin/gateway-data \
  --platform-url localhost:9090 \
  --namespace default \
  --enroll-token-file torana-enroll-dev-secret \
  --listen-http :8000 \
  --listen-grpc :8001
```

The gateway will immediately enroll, open a bidirectional control stream, receive the Ed25519-signed snapshot, send an `Ack`, and appear live in the **Fleet Gateways** tab of the React console.

---

## 4. REST API & SSE Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/status` | System health, Ed25519 public key, active version, fleet summary |
| `GET` | `/api/nodes` | List of registered data plane nodes with live telemetry |
| `GET` | `/api/config` | Current configuration schema (routes, upstreams, policies) |
| `GET` | `/api/config/history` | Historical snapshot records for rollback |
| `POST` | `/api/config/publish` | Sign new snapshot with Ed25519 and broadcast to fleet |
| `POST` | `/api/config/rollback` | Rollback to target version and broadcast to fleet |
| `GET` | `/api/revocations` | Active revoked API keys and tokens |
| `POST` | `/api/revocations` | Revoke credentials and push live Revocation to all nodes |
| `GET` | `/api/usage` | Aggregated non-sensitive token usage and request stats |
| `GET` | `/api/audit` | Append-only security audit trail |
| `GET` | `/api/events` | Server-Sent Events (SSE) stream for real-time UI updates |
