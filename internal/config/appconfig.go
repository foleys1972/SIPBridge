package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type RootConfig struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	Spec       Spec     `yaml:"spec" json:"spec"`
}

type Metadata struct {
	Name string `yaml:"name" json:"name"`
}

type Spec struct {
	Routes           []Route           `yaml:"routes" json:"routes"`
	Bridges          []Bridge          `yaml:"bridges" json:"bridges"`
	ConferenceGroups []ConferenceGroup `yaml:"conferenceGroups" json:"conferenceGroups"`
	HootGroups       []HootGroup       `yaml:"hootGroups" json:"hootGroups"`
	Users            []User            `yaml:"users" json:"users"`
	IVR              IVRConfig         `yaml:"ivr" json:"ivr"`
	// SIPStack persists SBC / TLS / listener overrides (merged with env at process start).
	SIPStack *SIPStackSpec `yaml:"sipStack,omitempty" json:"sipStack,omitempty"`
	// Servers lists other SIPBridge instances (API base URLs) for operations / probing.
	Servers []ManagedServer `yaml:"servers,omitempty" json:"servers,omitempty"`
	// Cluster is optional capacity / overflow settings (merged with SIPBRIDGE_* env at startup).
	Cluster *ClusterSpec `yaml:"cluster,omitempty" json:"cluster,omitempty"`
	// Database is optional enterprise config storage / PostgreSQL connection metadata (non-secret fields only).
	Database *DatabaseSpec `yaml:"database,omitempty" json:"database,omitempty"`
	// Recording holds global SIPREC / recorder integration (metadata and trunk hints).
	Recording *RecordingSpec `yaml:"recording,omitempty" json:"recording,omitempty"`
	// Capture controls local on-disk audio capture (WAV + metadata JSON per call).
	Capture *CaptureSpec `yaml:"capture,omitempty" json:"capture,omitempty"`
	// IPTVSources lists multicast RTP streams that can be injected into conference media.
	IPTVSources []IPTVSourceSpec `yaml:"iptvSources,omitempty" json:"iptvSources,omitempty"`
	// Auth configures local and AD LDS authentication + RBAC role mapping.
	Auth *AuthSpec `yaml:"auth,omitempty" json:"auth,omitempty"`
	// SIPTrunks are reusable outbound SBC/carrier profiles with optional per-trunk TLS certs.
	SIPTrunks []SIPTrunkSpec `yaml:"sipTrunks,omitempty" json:"sipTrunks,omitempty"`
	// DialPlan chooses outbound trunks based on target URI patterns.
	DialPlan []DialPlanRule `yaml:"dialPlan,omitempty" json:"dialPlan,omitempty"`
}

// ManagedServer is a peer SIPBridge control-plane endpoint (not a SIP trunk).
// The HTTP API URL is used for operations/probing only. SIP ingress and interconnect URIs are
// documentation hints: they do not automatically wire SIP between every pair of nodes; use your
// SBC/carrier trunks or bridge participants for peer-to-peer media paths.
type ManagedServer struct {
	ID            string `yaml:"id" json:"id"`
	Name          string `yaml:"name" json:"name"`
	APIBaseURL    string `yaml:"api_base_url" json:"api_base_url"`
	Region        string `yaml:"region,omitempty" json:"region,omitempty"`
	TLSSkipVerify bool   `yaml:"tls_skip_verify,omitempty" json:"tls_skip_verify,omitempty"`
	// SIPIngressURI is where external signaling (LB/VIP/DNS) should target new calls for this node—one value per node.
	SIPIngressURI string `yaml:"sip_ingress_uri,omitempty" json:"sip_ingress_uri,omitempty"`
	// InterconnectSIPURI is how other sites can reach this peer over SIP (often via SBC); optional; not a full mesh.
	InterconnectSIPURI string `yaml:"interconnect_sip_uri,omitempty" json:"interconnect_sip_uri,omitempty"`
	// CapacityWeight is a 1–100 hint for load balancers (not enforced by SIPBridge).
	CapacityWeight int `yaml:"capacity_weight,omitempty" json:"capacity_weight,omitempty"`
}

type Route struct {
	MatchUser  string `yaml:"match_user" json:"match_user"`
	TargetKind string `yaml:"target_kind" json:"target_kind"`
	TargetID   string `yaml:"target_id" json:"target_id"`
}

type Bridge struct {
	ID                 string        `yaml:"id" json:"id"`
	Name               string        `yaml:"name" json:"name"`
	Type               string        `yaml:"type" json:"type"`
	RingTimeoutSeconds int           `yaml:"ring_timeout_seconds" json:"ring_timeout_seconds"`
	DDIAccessEnabled   bool          `yaml:"ddi_access_enabled" json:"ddi_access_enabled"`
	DDIAccessNumber    string        `yaml:"ddi_access_number" json:"ddi_access_number"`
	Participants       []Participant `yaml:"participants" json:"participants"`
	// LineLabel is an optional SIPREC metadata label for this bridge (e.g. private wire / circuit id).
	LineLabel string `yaml:"line_label,omitempty" json:"line_label,omitempty"`
	// RecordingEnabled: nil = record when global SIPREC is on (legacy configs without key); false disables SIPREC for this bridge.
	RecordingEnabled *bool `yaml:"recording_enabled,omitempty" json:"recording_enabled,omitempty"`
}

type User struct {
	// ID is the primary identifier for the person (e.g. bank employee id). It is not auto-generated.
	ID            string `yaml:"id" json:"id"`
	DisplayName   string `yaml:"display_name" json:"display_name"`
	ParticipantID string `yaml:"participant_id" json:"participant_id"`
	// Region is the caller's preferred region label (e.g. EMEA, LDN); used to order outbound ring targets
	// to endpoints with the same Location / region first to reduce latency.
	Region                    string   `yaml:"region,omitempty" json:"region,omitempty"`
	AllowedBridgeIDs          []string `yaml:"allowed_bridge_ids" json:"allowed_bridge_ids"`
	AllowedConferenceGroupIDs []string `yaml:"allowed_conference_group_ids" json:"allowed_conference_group_ids"`
	// RecordingOptIn: when true, SIPREC may run for this user's calls if conference/bridge-wide recording is off — requires IVR PIN match, or a conference endpoint linked_user_id whose SIP URI matches the caller.
	RecordingOptIn bool         `yaml:"recording_opt_in,omitempty" json:"recording_opt_in,omitempty"`
	Devices        []UserDevice `yaml:"devices,omitempty" json:"devices,omitempty"`
}

type IVRConfig struct {
	EntryUser string `yaml:"entry_user" json:"entry_user"`
}

type Participant struct {
	ID          string `yaml:"id" json:"id"`
	DisplayName string `yaml:"display_name" json:"display_name"`
	SIPURI      string `yaml:"sip_uri" json:"sip_uri"`
	PairID      string `yaml:"pair_id" json:"pair_id"`
	End         string `yaml:"end" json:"end"`
	Role        string `yaml:"role" json:"role"`
	Location    string `yaml:"location" json:"location"`
	// LineLabel overrides bridge LineLabel for this leg in SIPREC metadata (e.g. private wire label).
	LineLabel string `yaml:"line_label,omitempty" json:"line_label,omitempty"`
}

type ConferenceGroup struct {
	ID                       string     `yaml:"id" json:"id"`
	Name                     string     `yaml:"name" json:"name"`
	Type                     string     `yaml:"type,omitempty" json:"type,omitempty"`
	RingTimeoutSeconds       int        `yaml:"ring_timeout_seconds" json:"ring_timeout_seconds"`
	WinnerKeepRingingSeconds int        `yaml:"winner_keep_ringing_seconds" json:"winner_keep_ringing_seconds"`
	DDIAccessEnabled         bool       `yaml:"ddi_access_enabled" json:"ddi_access_enabled"`
	DDIAccessNumber          string     `yaml:"ddi_access_number" json:"ddi_access_number"`
	SideA                    []Endpoint `yaml:"sideA" json:"sideA"`
	SideB                    []Endpoint `yaml:"sideB" json:"sideB"`
	// LineLabel is optional SIPREC metadata (e.g. service / circuit label for this conference).
	LineLabel string `yaml:"line_label,omitempty" json:"line_label,omitempty"`
	// RecordingEnabled: nil = record when global SIPREC is on (same default as bridges; legacy configs without key); false disables SIPREC for this conference group.
	RecordingEnabled *bool `yaml:"recording_enabled,omitempty" json:"recording_enabled,omitempty"`
	// IPTVSourceIDs links multicast IPTV sources to this conference/hoot group.
	IPTVSourceIDs []string `yaml:"iptv_source_ids,omitempty" json:"iptv_source_ids,omitempty"`
}

type HootGroup struct {
	ID                 string     `yaml:"id" json:"id"`
	Name               string     `yaml:"name" json:"name"`
	RingTimeoutSeconds int        `yaml:"ring_timeout_seconds" json:"ring_timeout_seconds"`
	DDIAccessEnabled   bool       `yaml:"ddi_access_enabled" json:"ddi_access_enabled"`
	DDIAccessNumber    string     `yaml:"ddi_access_number" json:"ddi_access_number"`
	RecordingEnabled   *bool      `yaml:"recording_enabled,omitempty" json:"recording_enabled,omitempty"`
	Talkers            []Endpoint `yaml:"talkers" json:"talkers"`
	Listeners          []Endpoint `yaml:"listeners" json:"listeners"`
}

type Endpoint struct {
	ID          string `yaml:"id" json:"id"`
	DisplayName string `yaml:"display_name" json:"display_name"`
	SIPURI      string `yaml:"sip_uri" json:"sip_uri"`
	Location    string `yaml:"location" json:"location"`
	// LinkedUserID / LinkedDeviceID tie this leg to a user device for SIPREC CTI metadata.
	LinkedUserID   string `yaml:"linked_user_id,omitempty" json:"linked_user_id,omitempty"`
	LinkedDeviceID string `yaml:"linked_device_id,omitempty" json:"linked_device_id,omitempty"`
	// LineLabel is optional SIPREC metadata for this conference endpoint (e.g. private wire leg).
	LineLabel string `yaml:"line_label,omitempty" json:"line_label,omitempty"`
}

// ValidateManagedServers checks inventory entries for multi-server operations console.
func ValidateManagedServers(servers []ManagedServer) error {
	seen := make(map[string]struct{})
	for _, s := range servers {
		id := strings.TrimSpace(s.ID)
		if id == "" {
			return fmt.Errorf("server id is required")
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate server id %q", id)
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("server %q: name is required", id)
		}
		raw := strings.TrimSpace(s.APIBaseURL)
		if raw == "" {
			return fmt.Errorf("server %q: api_base_url is required", id)
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("server %q: api_base_url: %w", id, err)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("server %q: api_base_url must use http or https", id)
		}
		if parsed.Host == "" {
			return fmt.Errorf("server %q: api_base_url must include a host", id)
		}
		if s.CapacityWeight != 0 && (s.CapacityWeight < 1 || s.CapacityWeight > 100) {
			return fmt.Errorf("server %q: capacity_weight must be between 1 and 100 or 0", id)
		}
		for _, raw := range []string{s.SIPIngressURI, s.InterconnectSIPURI} {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			u, err := url.Parse(raw)
			if err != nil || u.Scheme != "sip" {
				return fmt.Errorf("server %q: sip uri fields must be sip: URIs", id)
			}
		}
	}
	return nil
}

// ApplyVersionedRootDefaults normalizes nil slices after unmarshaling versioned YAML.
func ApplyVersionedRootDefaults(root *RootConfig) {
	if root.Spec.Routes == nil {
		root.Spec.Routes = []Route{}
	}
	if root.Spec.Bridges == nil {
		root.Spec.Bridges = []Bridge{}
	}
	if root.Spec.ConferenceGroups == nil {
		root.Spec.ConferenceGroups = []ConferenceGroup{}
	}
	if root.Spec.HootGroups == nil {
		root.Spec.HootGroups = []HootGroup{}
	}
	if root.Spec.Users == nil {
		root.Spec.Users = []User{}
	}
	if root.Spec.Servers == nil {
		root.Spec.Servers = []ManagedServer{}
	}
	if root.Spec.IPTVSources == nil {
		root.Spec.IPTVSources = []IPTVSourceSpec{}
	}
	if root.Spec.SIPTrunks == nil {
		root.Spec.SIPTrunks = []SIPTrunkSpec{}
	}
	if root.Spec.DialPlan == nil {
		root.Spec.DialPlan = []DialPlanRule{}
	}
	for i := range root.Spec.Users {
		if root.Spec.Users[i].AllowedBridgeIDs == nil {
			root.Spec.Users[i].AllowedBridgeIDs = []string{}
		}
		if root.Spec.Users[i].AllowedConferenceGroupIDs == nil {
			root.Spec.Users[i].AllowedConferenceGroupIDs = []string{}
		}
		if root.Spec.Users[i].Devices == nil {
			root.Spec.Users[i].Devices = []UserDevice{}
		}
	}
	if root.Spec.Database != nil && strings.TrimSpace(root.Spec.Database.ConfigStorage) == "" {
		root.Spec.Database.ConfigStorage = "yaml"
	}
}

func LoadAppConfig(path string) (RootConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return RootConfig{}, fmt.Errorf("read config file: %w", err)
	}

	// First try the versioned root config.
	var root RootConfig
	if err := yaml.Unmarshal(b, &root); err == nil {
		if root.APIVersion != "" || root.Kind != "" {
			ApplyVersionedRootDefaults(&root)
			return root, nil
		}
	}

	// Legacy compatibility: allow old-style top-level {routes, bridges}.
	type legacy struct {
		Routes []struct {
			MatchUser string `yaml:"match_user"`
			BridgeID  string `yaml:"bridge_id"`
		} `yaml:"routes"`
		Bridges []Bridge `yaml:"bridges"`
	}
	var l legacy
	if err := yaml.Unmarshal(b, &l); err != nil {
		return RootConfig{}, fmt.Errorf("parse yaml: %w", err)
	}
	root = RootConfig{
		APIVersion: "sipbridge.io/v1alpha1",
		Kind:       "SIPBridgeConfig",
		Metadata:   Metadata{Name: "legacy"},
		Spec:       Spec{Bridges: l.Bridges},
	}
	for _, r := range l.Routes {
		root.Spec.Routes = append(root.Spec.Routes, Route{
			MatchUser:  r.MatchUser,
			TargetKind: "bridge",
			TargetID:   r.BridgeID,
		})
	}
	ApplyVersionedRootDefaults(&root)
	return root, nil
}
