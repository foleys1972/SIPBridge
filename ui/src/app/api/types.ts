export type Healthz = { ok: boolean }

export type SIPStats = {
  started: boolean
  packets_rx: number
  bytes_rx: number
  last_packet_at: string
}

export type Endpoint = {
  id: string
  display_name: string
  sip_uri: string
  location?: string
  /** Employee id of user whose device this leg represents (SIPREC CTI). */
  linked_user_id?: string
  /** User device id from Settings → Users (must match that user’s devices). */
  linked_device_id?: string
}

export type ConferenceGroup = {
  id: string
  name: string
  type?: 'mrd' | 'ard' | 'hoot'
  ring_timeout_seconds: number
  winner_keep_ringing_seconds?: number
  ddi_access_enabled?: boolean
  ddi_access_number?: string
  /** When true and global recording is on, eligible calls may fork to SIPREC. */
  recording_enabled?: boolean
  iptv_source_ids?: string[]
  sideA?: Endpoint[]
  sideB?: Endpoint[]
}

export type IPTVSource = {
  id: string
  name?: string
  multicast_ip: string
  port: number
  payload_type?: number
  extract_audio_from_video?: boolean
  jitter_buffer_ms?: number
  enabled?: boolean
}

export type IPTVSubscriptionStatus = {
  source_id: string
  running: boolean
  audio_packets?: number
  dropped_packets?: number
  last_audio_at?: string
  started_at?: string
  ffmpeg_path?: string
  ffmpeg_error?: string
  ffmpeg_found?: boolean
}

export type HootGroup = {
  id: string
  name: string
  talkers?: Endpoint[]
  listeners?: Endpoint[]
}

export type BridgeParticipant = {
  id: string
  display_name: string
  sip_uri: string
  location?: string
  pair_id?: string
  end?: string
}

export type Bridge = {
  id: string
  name: string
  type?: string
  ddi_access_enabled?: boolean
  ddi_access_number?: string
  /** When false, SIPREC is not forked for this bridge. Omitted/true = record (when global SIPREC is on). */
  recording_enabled?: boolean
  participants: BridgeParticipant[]
}

export type UserDevice = {
  id: string
  kind: 'ddi' | 'mobile' | 'private_wire'
  address?: string
  /** CTI key/values passed in SIPREC metadata (extension, desk, trader id, etc.). */
  cti?: Record<string, string>
}

export type User = {
  id: string
  display_name?: string
  participant_id: string
  /** Preferred region for routing (matches endpoint location labels, e.g. EMEA, LDN). */
  region?: string
  allowed_bridge_ids: string[]
  allowed_conference_group_ids?: string[]
  recording_opt_in?: boolean
  devices?: UserDevice[]
}

export type IVRConfig = {
  entry_user: string
}

export type BridgeCallInfo = {
  bridge_id: string
  call_id: string
  from_tag: string
  to_tag: string
  from_uri: string
  to_uri: string
  contact_uri: string
  remote_addr: string
  created_at: string
  /** Present when the leg joined via IVR with PIN auth. */
  user_id?: string
  /** Same as user_id; config user id (e.g. bank employee id). */
  employee_id?: string
  display_name?: string
  pin_masked?: string
}

export type MIAttendanceRow = {
  bridge_id: string
  call_id: string
  user_id?: string
  employee_id?: string
  display_name?: string
  pin_masked?: string
  remote_addr: string
  created_at: string
}

export type MIAttendanceResponse = {
  attendance: MIAttendanceRow[]
}

export type ConferenceGroupLiveSession = {
  group_id: string
  source: 'ivr' | 'direct_invite'
  phase: string
  session_ref: string
  caller_side?: string
  conference_group_type?: string
  fanout_legs: number
  winner_established: boolean
  created_at: string
  remote_addr?: string
  preferred_region?: string
}

export type ConferenceGroupsUsageResponse = {
  sessions: ConferenceGroupLiveSession[]
  by_group: Record<string, number>
}

export type UserSummary = {
  id: string
  /** Same as id; bank / HR employee identifier (never auto-generated). */
  employee_id?: string
  display_name?: string
  region?: string
  pin_set: boolean
  recording_opt_in?: boolean
  device_count?: number
}

export type UsersListResponse = {
  users: UserSummary[]
}

export type UserDetail = {
  id: string
  employee_id?: string
  display_name?: string
  region?: string
  pin_masked?: string
  pin_set: boolean
  recording_opt_in?: boolean
  devices?: UserDevice[]
  allowed_bridge_ids: string[]
  allowed_conference_group_ids?: string[]
}

export type UserDetailResponse = {
  user: UserDetail
}

export type Route = {
  match_user: string
  target_kind: string
  target_id: string
}

/** Saved in config.yaml as spec.sipStack; overrides env at startup. */
export type SIPStackSpec = {
  bind_addr?: string | null
  udp_port?: number | null
  outbound_proxy_addr?: string | null
  outbound_proxy_port?: number | null
  outbound_transport?: 'udp' | 'tls' | string | null
  advertise_addr?: string | null
  tls_root_ca_file?: string | null
  tls_client_cert_file?: string | null
  tls_client_key_file?: string | null
  tls_insecure_skip_verify?: boolean | null
  tls_server_name?: string | null
  session_timer_enabled?: boolean | null
}

/** Effective runtime SIP stack (GET /v1/settings/sip → effective). */
export type SIPConfig = {
  bind_addr: string
  udp_port: number
  outbound_proxy_addr: string
  outbound_proxy_port: number
  outbound_transport: string
  advertise_addr: string
  tls_root_ca_file: string
  tls_client_cert_file: string
  tls_client_key_file: string
  tls_insecure_skip_verify: boolean
  tls_server_name: string
  session_timer_enabled: boolean
}

export type BridgeListItem = {
  id: string
  name: string
  type?: string
  active_calls: number
  participants: BridgeParticipant[]
}

export type BridgeListResponse = {
  bridges: BridgeListItem[]
}

export type BridgeDetailResponse = {
  bridge: Bridge
  calls: BridgeCallInfo[]
}

export type SIPSettingsResponse = {
  effective: SIPConfig
  saved: SIPStackSpec | null
  note: string
}

/** Peer SIPBridge instances for the operations console (control-plane URLs, not SIP trunks). */
export type ManagedServer = {
  id: string
  name: string
  api_base_url: string
  region?: string
  tls_skip_verify?: boolean
  sip_ingress_uri?: string
  interconnect_sip_uri?: string
  capacity_weight?: number
}

export type ServerProbe = {
  ok: boolean
  latency_ms: number
  error: string
}

export type ServersListResponse = {
  local_instance_id: string
  servers: ManagedServer[]
}

export type ServersProbedRow = ManagedServer & {
  probe: ServerProbe
}

export type ServersProbeResponse = {
  local_instance_id: string
  servers: ServersProbedRow[]
}

export type ClusterConfigResponse = {
  effective?: {
    max_concurrent_calls: number
    soft_max_concurrent_calls: number
    overflow_redirect_enabled: boolean
    overflow_redirect_sip_uri: string
  }
  saved?: unknown
  note?: string
}

export type ClusterSummaryResponse = {
  local: Record<string, unknown>
  peers: Record<string, unknown>[]
}

export type ConfigStatus = {
  config_path: string
  config_http_url: string
  config_read_only: boolean
  config_http_poll_sec: number
}

/** Declarative enterprise config storage (secrets via env, not YAML). */
export type PostgresSpec = {
  host: string
  port: number
  user: string
  database: string
  ssl_mode?: string
  password_env_var?: string
  schema?: string
}

export type DatabaseSpec = {
  config_storage: 'yaml' | 'http' | 'postgres'
  postgres?: PostgresSpec
}

export type DatabaseSettingsResponse = {
  saved: DatabaseSpec | null
  env: {
    config_http_url_set: boolean
    database_url_set: boolean
  }
  note: string
}

export type RecordingTrunkEntry = {
  id: string
  label?: string
  recorder_sip_uri?: string
  recording_trunk_sip_uri?: string
}

export type SIPRECIntegrationSpec = {
  enabled: boolean
  /** Legacy single recorder when trunks is empty. */
  recorder_sip_uri?: string
  recording_trunk_sip_uri?: string
  metadata_namespace?: string
  /** Multiple regional recorders/trunks (e.g. US vs EMEA). */
  trunks?: RecordingTrunkEntry[]
  default_trunk_id?: string
  /** Maps user region labels (User.region) to trunk ids. */
  region_to_trunk?: Record<string, string>
}

export type RecordingSpec = {
  global_enabled: boolean
  siprec?: SIPRECIntegrationSpec
}

export type RecordingSettingsResponse = {
  saved: RecordingSpec | null
  note: string
}

export type CaptureSpec = {
  enabled: boolean
  directory?: string
  capture_bridges?: boolean
  capture_conferences?: boolean
}

export type SIPTrunkSpec = {
  id: string
  name?: string
  proxy_addr: string
  proxy_port: number
  transport?: 'udp' | 'tls' | string
  tls_root_ca_file?: string
  tls_client_cert_file?: string
  tls_client_key_file?: string
  tls_insecure_skip_verify?: boolean
  tls_server_name?: string
}

export type DialPlanRule = {
  id: string
  enabled?: boolean
  user_prefix?: string
  domain?: string
  uri_regex?: string
  target_trunk_id: string
}

export type AuthRole = 'admin' | 'operator' | 'readonly'

export type LocalAuthUser = {
  username: string
  password: string
  role: AuthRole
}

export type LocalAuthSpec = {
  enabled: boolean
  users?: LocalAuthUser[]
}

export type ADLDSSpec = {
  enabled: boolean
  url?: string
  bind_dn?: string
  bind_password_env_var?: string
  base_dn?: string
  user_filter?: string
  group_role_map?: Record<string, AuthRole>
}

export type AuthSpec = {
  enabled: boolean
  session_ttl_minutes?: number
  local?: LocalAuthSpec
  adlds?: ADLDSSpec
}

/** Result of POST /v1/settings/recording/test (SIP OPTIONS toward recorder). */
export type SIPRECProbeResult = {
  ok: boolean
  reachable?: boolean
  target_uri?: string
  destination?: string
  error?: string
  sip_status?: number
  reason?: string
  response_preview?: string
  roundtrip_ms?: number
  step?: string
  /** Explains common SIP codes (e.g. 503 + drachtio without Node on 9022). */
  hint?: string
}

export type ServiceDashboardRow = {
  id: string
  label: string
  status: string
  detail?: string
  latency_ms?: number
}

export type ServiceDashboardLogEntry = {
  ts: string
  level: string
  message: string
}

/** SIP Bridge fetches SIPREC GET /api/health/dashboard when SIPREC_RECORDER_BASE_URL is set (default localhost:3030). */
export type RecorderDashboardSnapshot = {
  ok: boolean
  url?: string
  error?: string
  latency_ms?: number
  /** Full JSON from SIPREC (services, bridge, log, …). */
  payload?: Record<string, unknown>
}

export type ServiceDashboardResponse = {
  checked_at: string
  summary_up: number
  summary_total: number
  services: ServiceDashboardRow[]
  log: ServiceDashboardLogEntry[]
  note: string
  recorder?: RecorderDashboardSnapshot | null
}

/** Saved spec.cluster (partial); merged with env at startup. */
export type ClusterSpec = {
  max_concurrent_calls?: number | null
  soft_max_concurrent_calls?: number | null
  overflow_redirect_enabled?: boolean | null
  overflow_redirect_sip_uri?: string | null
}

export type ClusterLimits = {
  max_concurrent_calls: number
  soft_max_concurrent_calls: number
  overflow_redirect_enabled: boolean
  overflow_redirect_sip_uri: string
}

export type ClusterSettingsResponse = {
  saved: ClusterSpec | null
  effective: ClusterLimits
  note: string
}

export type RootConfig = {
  apiVersion: string
  kind: string
  metadata: { name: string }
  spec: {
    routes: Route[]
    bridges: Bridge[]
    conferenceGroups: ConferenceGroup[]
    hootGroups: HootGroup[]
    users?: User[]
    ivr?: IVRConfig
    sipStack?: SIPStackSpec
    servers?: ManagedServer[]
    cluster?: unknown
    database?: DatabaseSpec
    recording?: RecordingSpec
    iptvSources?: IPTVSource[]
    capture?: CaptureSpec
    sipTrunks?: SIPTrunkSpec[]
    dialPlan?: DialPlanRule[]
    auth?: AuthSpec
  }
}
