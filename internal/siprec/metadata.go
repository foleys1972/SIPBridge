package siprec

import (
	"encoding/xml"
	"strings"
)

// ParticipantRecordingMeta is the logical payload merged from user, device, conference, and dial-in context
// for SIPREC metadata XML toward a recorder (RFC 7866-style participant data).
type ParticipantRecordingMeta struct {
	EmployeeID  string
	DisplayName string
	// ParticipantPIN is the IVR access PIN / participant_id digits the user entered (identifies them in metadata).
	ParticipantPIN    string
	ConferenceGroupID string
	ConferenceName    string
	BridgeID          string
	BridgeName        string
	// LineLabel identifies a private wire / bridge leg / conference line for operators (see bridge line_label).
	LineLabel     string
	DialIn        bool
	DeviceKind    string
	DeviceAddress string
	DeviceID      string
	CTI           map[string]string
}

type participantXML struct {
	XMLName        xml.Name `xml:"participant"`
	EmployeeID     string   `xml:"employee_id,omitempty"`
	DisplayName    string   `xml:"display_name,omitempty"`
	ParticipantPIN string   `xml:"participant_pin,omitempty"`
	ConferenceID   string   `xml:"conference_group_id,omitempty"`
	ConferenceName string   `xml:"conference_name,omitempty"`
	BridgeID       string   `xml:"bridge_id,omitempty"`
	BridgeName     string   `xml:"bridge_name,omitempty"`
	LineLabel      string   `xml:"line_label,omitempty"`
	DialIn         string   `xml:"dial_in,omitempty"`
	DeviceKind     string   `xml:"device_kind,omitempty"`
	DeviceID       string   `xml:"device_id,omitempty"`
	DeviceAddress  string   `xml:"device_address,omitempty"`
	CTI            []kvXML  `xml:"cti>entry,omitempty"`
}

type kvXML struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}

// BuildMetadataXML returns a minimal XML document for SIPREC metadata attachment (illustrative).
func BuildMetadataXML(m ParticipantRecordingMeta) (string, error) {
	p := participantXML{
		EmployeeID:     strings.TrimSpace(m.EmployeeID),
		DisplayName:    strings.TrimSpace(m.DisplayName),
		ParticipantPIN: strings.TrimSpace(m.ParticipantPIN),
		ConferenceID:   strings.TrimSpace(m.ConferenceGroupID),
		ConferenceName: strings.TrimSpace(m.ConferenceName),
		BridgeID:       strings.TrimSpace(m.BridgeID),
		BridgeName:     strings.TrimSpace(m.BridgeName),
		LineLabel:      strings.TrimSpace(m.LineLabel),
		DeviceKind:     strings.TrimSpace(m.DeviceKind),
		DeviceID:       strings.TrimSpace(m.DeviceID),
		DeviceAddress:  strings.TrimSpace(m.DeviceAddress),
	}
	if m.DialIn {
		p.DialIn = "true"
	}
	for k, v := range m.CTI {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		p.CTI = append(p.CTI, kvXML{Key: k, Value: v})
	}
	b, err := xml.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
