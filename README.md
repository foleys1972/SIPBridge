# SIPBridge

SIPBridge is a SIP conferencing bridge designed for financial trading floors and private wire networks. It provides seamless connectivity between private wires, PSTN/mobile lines, and SIP endpoints, supporting Automatic Ring Down (ARD), Manual Ring Down (MRD), and HOOT (open talk/listen) call modes with full SIPREC voice recording.

## Call Types

### ARD — Automatic Ring Down

Participants on a private wire dial a conference group. If the opposite side is already present, the joining party is connected immediately without re-ringing. If the opposite side is absent, SIPBridge fans out an INVITE to all opposing endpoints and bridges both legs once answered. Identified by `type: ard` in the conference group config.

### MRD — Manual Ring Down

Participants dial into a conference group and must press DTMF **`9`** to ring the opposing side. Configurable `ring_timeout_seconds` controls how long SIPBridge waits for the far side to answer, and `winner_keep_ringing_seconds` keeps the call ringing after one remote leg has answered. Default mode when `type` is unset or set to `mrd`.

### HOOT — Hold On Other Talk

An open always-on talk/listen group. When a participant joins, SIPBridge fans out INVITEs to **all** endpoints across both SideA and SideB. Everyone who answers is bridged into a shared session where all participants can hear and speak simultaneously. Identified by `type: hoot`.

### IVR Entry

A single DDI entry point (configured as `ivr.entry_user`) that accepts any inbound call and challenges the caller with a PIN. The PIN maps to a `user.participant_id` to authenticate the caller and associate them with a bridge or conference group. Supports all three call modes once authenticated.

## PSTN and Mobile Access

Any endpoint, including PSTN numbers and mobile devices, can participate as a conference group endpoint or bridge participant. Configure with a `sip_uri` pointing to your SBC or PSTN gateway trunk. Users can be assigned `devices` of kind `ddi`, `mobile`, or `private_wire` to associate incoming or outgoing legs with employee identity for recording and audit purposes.

## Voice Recording — SIPREC

SIPBridge implements RFC 7866 SIPREC recording. When recording is enabled, SIPBridge sends a multipart SIP INVITE to a recording server carrying both the RTP media description and a rich XML metadata document. Recording is configured at three levels:

- **Global**: `spec.recording.siprec.enabled` enables recording for all calls by default.
- **Per bridge/conference**: `recording_enabled: true/false` overrides the global setting for a specific bridge or conference group. `null` (omitted) inherits the global.
- **User opt-in**: A user with `recording_opt_in: true` can be recorded even when the conference/bridge recording is off, provided they authenticate via IVR PIN or a linked device.

Multiple recording trunks are supported for regional redundancy. The `region_to_trunk` map routes each call to the nearest recorder based on `Endpoint.location` or `User.region`.

SIPREC metadata includes employee ID, display name, IVR PIN (masked), device kind, device address, private wire line labels, conference/bridge identity, and arbitrary CTI key/value pairs from the user device configuration. This makes the recorder payload suitable for trading floor compliance and surveillance platforms.

## Configuration

Configuration is a versioned YAML file following Kubernetes object conventions.

```yaml
apiVersion: sipbridge.io/v1alpha1
kind: SIPBridgeConfig
metadata:
  name: default
spec:
  routes: []           # Maps incoming SIP addresses to bridges, conferences, or IVR
  bridges: []          # Point-to-point or multi-party bridge rooms
  conferenceGroups: [] # ARD / MRD / HOOT groups with SideA + SideB endpoints
  hootGroups: []       # Legacy standalone HOOT groups (talkers + listeners)
  users: []            # Employee records with PIN, devices, CTI, and recording opt-in
  ivr:
    entry_user: "9000"
  sipStack: {}         # SIP transport, TLS, SBC proxy, and session timer settings
  servers: []          # Peer SIPBridge nodes for multi-site deployments
  cluster: {}          # Concurrent call limits and overflow redirect
  database: {}         # Optional config backend (YAML file / HTTP / PostgreSQL)
  recording:
    global_enabled: false
    siprec:
      enabled: false
      trunks: []
      region_to_trunk: {}
```

### Conference Group Example

```yaml
spec:
  conferenceGroups:
    - id: conf-ard-001
      name: "Equities Desk A"
      type: ard                    # ard | mrd | hoot
      ring_timeout_seconds: 30
      ddi_access_enabled: true
      ddi_access_number: "2100"
      recording_enabled: true
      line_label: "Equities/NY-LDN"
      sideA:
        - id: a-1
          display_name: "Trader Smith"
          sip_uri: sip:2101@192.168.1.10
          location: LDN
          linked_user_id: emp-smith
      sideB:
        - id: b-1
          display_name: "Sales Jones"
          sip_uri: sip:2201@192.168.1.20
          location: NYC
          linked_user_id: emp-jones
```

### Recording Example

```yaml
spec:
  recording:
    global_enabled: true
    siprec:
      enabled: true
      trunks:
        - id: rec-ldn
          label: "London Recorder"
          recorder_sip_uri: sip:recorder@192.168.50.10
        - id: rec-nyc
          label: "New York Recorder"
          recorder_sip_uri: sip:recorder@192.168.50.20
      default_trunk_id: rec-ldn
      region_to_trunk:
        LDN: rec-ldn
        NYC: rec-nyc
```

### Local Audio Capture

Local capture writes WAV files and a companion metadata JSON file per call to disk — useful for triage and compliance where full SIPREC infrastructure is not available.

```yaml
spec:
  capture:
    enabled: true
    directory: captures          # relative to working directory (default: "captures")
    capture_bridges: true        # capture private wire / bridge calls
    capture_conferences: true    # capture conference group calls
```

Each call produces three files:

```
captures/2026-04-26T14-30-00_a1b2c3d4_inbound.wav   # audio from caller
captures/2026-04-26T14-30-00_a1b2c3d4_outbound.wav  # audio from remote leg
captures/2026-04-26T14-30-00_a1b2c3d4_meta.json     # call metadata
```

The metadata JSON includes call type, bridge/conference ID, participant details (user ID, display name, SIP URI, location, role), timestamps, duration, and per-file recording details.

Enable `LOG_DIR` alongside capture to keep rolling logs for post-call triage:

```bash
LOG_DIR=logs ./sipbridge
```

### SIP Stack Settings

```yaml
spec:
  sipStack:
    bind_addr: "0.0.0.0"
    udp_port: 5060
    advertise_addr: "10.0.1.50"   # NAT / routable IP for Contact, Via, SDP
    outbound_proxy_addr: "10.0.0.1"
    outbound_proxy_port: 5060
    outbound_transport: "udp"     # udp | tls
    session_timer_enabled: true   # RFC 4028 session refresh (recommended for SBC interop)
```

For SBC connectivity over SIPS/TLS:

```yaml
spec:
  sipStack:
    outbound_transport: tls
    outbound_proxy_addr: "10.0.0.1"
    outbound_proxy_port: 5061
    tls_root_ca_file: "/etc/sipbridge/ca.pem"      # CA that signed the SBC cert
    tls_client_cert_file: "/etc/sipbridge/cert.pem" # mTLS client cert (if SBC requires it)
    tls_client_key_file: "/etc/sipbridge/key.pem"
    tls_server_name: "sbc.example.com"              # SNI when cert CN differs from IP
```

## Environment Variables

| Variable | Description |
|---|---|
| `SIP_BIND_ADDR` | SIP listen address (default `0.0.0.0`) |
| `SIP_UDP_PORT` | SIP UDP port (default `5060`) |
| `SIP_ADVERTISE_ADDR` | Routable IP for Contact/Via/SDP behind NAT |
| `SIP_OUTBOUND_PROXY_ADDR` | SBC/proxy IP address |
| `SIP_OUTBOUND_PROXY_PORT` | SBC/proxy port |
| `SIP_OUTBOUND_TRANSPORT` | `udp` (default) or `tls` for SIPS to SBC |
| `SIP_TLS_ROOT_CA_FILE` | PEM CA cert for verifying the SBC's TLS cert |
| `SIP_TLS_CLIENT_CERT_FILE` | PEM client cert for mTLS to SBC |
| `SIP_TLS_CLIENT_KEY_FILE` | PEM private key for mTLS to SBC |
| `SIP_TLS_SERVER_NAME` | SNI override when SBC cert doesn't match its IP |
| `SIP_TLS_INSECURE_SKIP_VERIFY` | Skip SBC TLS cert verification (dev only) |
| `SIP_SESSION_TIMER_ENABLED` | Send Min-SE/Session-Expires on INVITE (default `true`) |
| `CONFIG_PATH` | Path to config.yaml (default `config.yaml`) |
| `CONFIG_HTTP_URL` | URL to fetch config from (GitOps / HTTP polling) |
| `CONFIG_HTTP_POLL_SECONDS` | Poll interval for remote config in seconds |
| `CONFIG_HTTP_BEARER_TOKEN` | Bearer token for remote config URL |
| `API_BIND_ADDR` | HTTP API listen address (default `127.0.0.1`) |
| `API_PORT` | HTTP API port (default `8081`) |
| `LOG_DIR` | Directory for rotating daily log files; empty = stdout only |

## Running

### Go service

```bash
go run ./cmd/sipbridge
```

Or with a specific config file:

```bash
CONFIG_FILE=config.yaml go run ./cmd/sipbridge
```

### Windows EXE package

Build a standalone Windows binary:

```powershell
go build -o .\dist\windows\sipbridge.exe .\cmd\sipbridge
```

Stage runtime files beside the executable:

```powershell
Copy-Item .\config.yaml .\dist\windows\config.yaml -Force
Copy-Item .\run-sipbridge.ps1 .\dist\windows\run-sipbridge.ps1 -Force
Copy-Item .\run-sipbridge.bat .\dist\windows\run-sipbridge.bat -Force
```

Optional zip for distribution:

```powershell
Compress-Archive -Path .\dist\windows\* -DestinationPath .\dist\SIPBridge-windows.zip -Force
```

### Web UI (development)

```bash
cd ui
npm install
npm run dev
```

The production UI is pre-built into `ui/dist/` and served by the Go service at `/`.

## Web Console

The built-in web console provides:

- **Overview** — service health and active call summary
- **Stack Health** — reachability status of recorders, SBC, and peer SIPBridge nodes
- **Bridges & Lines** — live view of all bridge rooms with active call counts; drop or reset participants
- **Conference Groups** — create and edit ARD/MRD/HOOT groups; view live fanout sessions
- **Users** — manage employee records, IVR PINs, devices, and recording opt-in flags
- **Recording Settings** — configure SIPREC trunks, regional routing, and probe recorder reachability
- **MI Dashboard** — call attendance and audit log with user/participant mapping
- **Cluster** — concurrent call limits, soft thresholds, and overflow redirect configuration
- **Servers** — multi-site peer registration and cross-node capacity view
- **Config Editor** — view, edit, and validate the full `config.yaml` in-browser

## Multi-Site Deployments

Multiple SIPBridge instances can be registered with each other via `spec.servers[]`. Each node exposes its capacity and active call count to the cluster summary API, allowing load-aware routing decisions and operational visibility across sites.

## Architecture Notes

- **SIP stack**: Pure Go UDP/TLS SIP implementation. No external SIP library dependency.
- **Media**: RTP relay with SDP offer/answer. Each bridged call is a pair of RTP sessions relayed by SIPBridge.
- **Config hot-reload**: File or HTTP-polled config is applied without service restart; active calls are not disrupted.
- **SIPREC**: Outbound SIPREC sessions run in parallel to the bridged call. Media is forked to the recorder as a separate RTP stream described in the SIPREC multipart INVITE.
- **Recording stubs**: `internal/recording/recorder.go` is a no-op placeholder. SIPREC is the supported recording path.
