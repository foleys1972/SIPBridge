package config

import (
	"fmt"
	"net/url"
	"strings"
)

// RecordingSpec holds global SIP recording / SIPREC integration settings (declarative).
type RecordingSpec struct {
	GlobalEnabled bool `yaml:"global_enabled" json:"global_enabled"`
	SIPREC        *SIPRECIntegrationSpec `yaml:"siprec,omitempty" json:"siprec,omitempty"`
}

// RecordingTrunkEntry is one regional (or logical) path to a SIPREC recorder and optional outbound trunk.
type RecordingTrunkEntry struct {
	ID    string `yaml:"id" json:"id"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
	// RecorderSIPURI is the sip: contact for the recorder (SIPREC RS) for this trunk.
	RecorderSIPURI string `yaml:"recorder_sip_uri,omitempty" json:"recorder_sip_uri,omitempty"`
	// RecordingTrunkSIPURI is an optional dedicated outbound trunk (sip:) toward the recorder/SBC path.
	RecordingTrunkSIPURI string `yaml:"recording_trunk_sip_uri,omitempty" json:"recording_trunk_sip_uri,omitempty"`
}

// SIPRECIntegrationSpec describes how to reach SIPREC recorders, optionally per region/trunk.
type SIPRECIntegrationSpec struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Legacy single recorder + trunk when Trunks is empty (backward compatible).
	RecorderSIPURI string `yaml:"recorder_sip_uri,omitempty" json:"recorder_sip_uri,omitempty"`
	RecordingTrunkSIPURI string `yaml:"recording_trunk_sip_uri,omitempty" json:"recording_trunk_sip_uri,omitempty"`
	// Trunks defines multiple recorders/trunks (e.g. US vs EMEA). When non-empty, use RegionToTrunk and DefaultTrunkID.
	Trunks []RecordingTrunkEntry `yaml:"trunks,omitempty" json:"trunks,omitempty"`
	// DefaultTrunkID must match a trunk id when Trunks is non-empty.
	DefaultTrunkID string `yaml:"default_trunk_id,omitempty" json:"default_trunk_id,omitempty"`
	// RegionToTrunk maps user region labels (see User.Region) to trunk ids, e.g. "US" -> "us".
	RegionToTrunk map[string]string `yaml:"region_to_trunk,omitempty" json:"region_to_trunk,omitempty"`
	// MetadataNamespace is an optional label for participant metadata (CTI) in XML.
	MetadataNamespace string `yaml:"metadata_namespace,omitempty" json:"metadata_namespace,omitempty"`
}

// UserDevice is a routable identity (DDI, mobile, private wire) with CTI key/values for SIPREC metadata.
type UserDevice struct {
	ID      string            `yaml:"id" json:"id"`
	Kind    string            `yaml:"kind" json:"kind"` // ddi, mobile, private_wire
	Address string            `yaml:"address,omitempty" json:"address,omitempty"`
	CTI     map[string]string `yaml:"cti,omitempty" json:"cti,omitempty"`
}

func mustBeSIPURI(field, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "sip" {
		return fmt.Errorf("%s must be a sip: URI", field)
	}
	return nil
}

// ValidateRecordingSpec validates spec.recording when present.
func ValidateRecordingSpec(r *RecordingSpec) error {
	if r == nil {
		return nil
	}
	if r.SIPREC != nil && r.SIPREC.Enabled {
		s := r.SIPREC
		if len(s.Trunks) > 0 {
			seen := make(map[string]struct{})
			for _, t := range s.Trunks {
				id := strings.TrimSpace(t.ID)
				if id == "" {
					return fmt.Errorf("spec.recording.siprec.trunks: each trunk requires id")
				}
				if _, ok := seen[id]; ok {
					return fmt.Errorf("spec.recording.siprec.trunks: duplicate trunk id %q", id)
				}
				seen[id] = struct{}{}
				if strings.TrimSpace(t.RecorderSIPURI) == "" {
					return fmt.Errorf("spec.recording.siprec.trunks[%q]: recorder_sip_uri is required", id)
				}
				if err := mustBeSIPURI("spec.recording.siprec.trunks["+id+"].recorder_sip_uri", t.RecorderSIPURI); err != nil {
					return err
				}
				if err := mustBeSIPURI("spec.recording.siprec.trunks["+id+"].recording_trunk_sip_uri", t.RecordingTrunkSIPURI); err != nil {
					return err
				}
			}
			def := strings.TrimSpace(s.DefaultTrunkID)
			if def == "" {
				return fmt.Errorf("spec.recording.siprec.default_trunk_id is required when siprec.trunks is non-empty")
			}
			if _, ok := seen[def]; !ok {
				return fmt.Errorf("spec.recording.siprec.default_trunk_id %q must match a trunk id", def)
			}
			for regionLabel, tid := range s.RegionToTrunk {
				tid = strings.TrimSpace(tid)
				if _, ok := seen[tid]; !ok {
					return fmt.Errorf("spec.recording.siprec.region_to_trunk[%q] references unknown trunk id %q", regionLabel, tid)
				}
			}
		} else {
			if err := mustBeSIPURI("spec.recording.siprec.recorder_sip_uri", s.RecorderSIPURI); err != nil {
				return err
			}
			if strings.TrimSpace(s.RecorderSIPURI) == "" {
				return fmt.Errorf("spec.recording.siprec.recorder_sip_uri is required when siprec.enabled is true (or add siprec.trunks)")
			}
			if err := mustBeSIPURI("spec.recording.siprec.recording_trunk_sip_uri", s.RecordingTrunkSIPURI); err != nil {
				return err
			}
		}
	}
	return nil
}

// SelectRecordingTrunkForRegion resolves which recorder/trunk to use from SIPREC settings and the user's region label.
// userRegion should match keys in RegionToTrunk (case-insensitive); otherwise DefaultTrunkID or legacy single URI applies.
func SelectRecordingTrunkForRegion(s *SIPRECIntegrationSpec, userRegion string) (RecordingTrunkEntry, bool) {
	if s == nil || !s.Enabled {
		return RecordingTrunkEntry{}, false
	}
	if len(s.Trunks) > 0 {
		byID := make(map[string]RecordingTrunkEntry, len(s.Trunks))
		for _, t := range s.Trunks {
			id := strings.TrimSpace(t.ID)
			if id != "" {
				byID[id] = t
			}
		}
		ur := strings.TrimSpace(userRegion)
		if ur != "" && s.RegionToTrunk != nil {
			if tid, ok := s.RegionToTrunk[ur]; ok {
				if t, ok := byID[strings.TrimSpace(tid)]; ok {
					return t, true
				}
			}
			for k, tid := range s.RegionToTrunk {
				if strings.EqualFold(strings.TrimSpace(k), ur) {
					if t, ok := byID[strings.TrimSpace(tid)]; ok {
						return t, true
					}
				}
			}
		}
		def := strings.TrimSpace(s.DefaultTrunkID)
		if def != "" {
			if t, ok := byID[def]; ok {
				return t, true
			}
		}
		if len(s.Trunks) > 0 {
			return s.Trunks[0], true
		}
	}
	// Legacy single recorder.
	if strings.TrimSpace(s.RecorderSIPURI) != "" {
		return RecordingTrunkEntry{
			ID:                   "legacy",
			RecorderSIPURI:       strings.TrimSpace(s.RecorderSIPURI),
			RecordingTrunkSIPURI: strings.TrimSpace(s.RecordingTrunkSIPURI),
		}, true
	}
	return RecordingTrunkEntry{}, false
}

// ValidateRecordingLinks checks conference endpoint links to users/devices.
func ValidateRecordingLinks(root RootConfig) error {
	userByID := make(map[string]*User)
	for i := range root.Spec.Users {
		u := &root.Spec.Users[i]
		userByID[u.ID] = u
	}
	deviceIDs := func(u *User) map[string]struct{} {
		m := make(map[string]struct{})
		for _, d := range u.Devices {
			id := strings.TrimSpace(d.ID)
			if id != "" {
				m[id] = struct{}{}
			}
		}
		return m
	}
	checkEp := func(gID string, ep Endpoint) error {
		lu := strings.TrimSpace(ep.LinkedUserID)
		ld := strings.TrimSpace(ep.LinkedDeviceID)
		if lu == "" && ld == "" {
			return nil
		}
		if lu == "" {
			return fmt.Errorf("conference group %q: endpoint %q has linked_device_id without linked_user_id", gID, ep.ID)
		}
		u, ok := userByID[lu]
		if !ok {
			return fmt.Errorf("conference group %q: linked_user_id %q not found", gID, lu)
		}
		if ld != "" {
			devs := deviceIDs(u)
			if _, ok := devs[ld]; !ok {
				return fmt.Errorf("conference group %q: linked_device_id %q not found on user %q", gID, ld, lu)
			}
		}
		return nil
	}
	for _, g := range root.Spec.ConferenceGroups {
		for _, ep := range g.SideA {
			if err := checkEp(g.ID, ep); err != nil {
				return err
			}
		}
		for _, ep := range g.SideB {
			if err := checkEp(g.ID, ep); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateUserDevices(devices []UserDevice) error {
	seen := make(map[string]struct{})
	for _, d := range devices {
		id := strings.TrimSpace(d.ID)
		if id == "" {
			return fmt.Errorf("user device id is required")
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate user device id %q", id)
		}
		seen[id] = struct{}{}
		k := strings.ToLower(strings.TrimSpace(d.Kind))
		switch k {
		case "ddi", "mobile", "private_wire", "":
		default:
			return fmt.Errorf("user device %q: kind must be ddi, mobile, or private_wire", id)
		}
	}
	return nil
}

// ValidateUserDeviceList validates devices on a user record.
func ValidateUserDeviceList(devices []UserDevice) error {
	return validateUserDevices(devices)
}
