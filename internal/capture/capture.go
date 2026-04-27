// Package capture provides local RTP audio capture to WAV files with a
// companion metadata JSON file per call.
//
// Each call produces up to three files in the capture directory:
//
//	<timestamp>_<shortKey>_inbound.wav   — caller leg (PT 0 PCMU or PT 8 PCMA decoded to 16-bit PCM)
//	<timestamp>_<shortKey>_outbound.wav  — far-end/peer leg
//	<timestamp>_<shortKey>_meta.json     — call metadata (written on Close)
package capture

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RTPPayloadType identifies the G.711 codec in use for a leg.
type RTPPayloadType uint8

const (
	PayloadPCMU RTPPayloadType = 0
	PayloadPCMA RTPPayloadType = 8
)

// ParticipantMeta holds identity information for one call leg.
type ParticipantMeta struct {
	Role        string `json:"role"` // "inbound" or "outbound"
	FromURI     string `json:"from_uri,omitempty"`
	FromHeader  string `json:"from_header,omitempty"`
	RemoteAddr  string `json:"remote_addr,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	DeviceID    string `json:"device_id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Location    string `json:"location,omitempty"`
	DeviceKind  string `json:"device_kind,omitempty"`
	LineLabel   string `json:"line_label,omitempty"`
}

// CallMeta is the identity and context of the call passed in at capture start.
type CallMeta struct {
	SessionKey          string           `json:"session_key"`
	SIPCallID           string           `json:"sip_call_id,omitempty"`
	CallType            string           `json:"call_type,omitempty"` // bridge | conference | hoot | ivr
	BridgeID            string           `json:"bridge_id,omitempty"`
	BridgeName          string           `json:"bridge_name,omitempty"`
	ConferenceGroupID   string           `json:"conference_group_id,omitempty"`
	ConferenceGroupName string           `json:"conference_group_name,omitempty"`
	LineLabel           string           `json:"line_label,omitempty"`
	Inbound             *ParticipantMeta `json:"inbound,omitempty"`
	Outbound            *ParticipantMeta `json:"outbound,omitempty"`
}

type recordingFile struct {
	File            string  `json:"file"`
	Leg             string  `json:"leg"`
	Codec           string  `json:"codec"`
	SampleRateHz    int     `json:"sample_rate_hz"`
	Channels        int     `json:"channels"`
	DurationSeconds float64 `json:"duration_seconds"`
	BytesWritten    uint32  `json:"bytes_written"`
}

type callRecord struct {
	CallMeta
	StartedAt       string          `json:"started_at"`
	EndedAt         string          `json:"ended_at,omitempty"`
	DurationSeconds float64         `json:"duration_seconds,omitempty"`
	Recordings      []recordingFile `json:"recordings"`
}

// leg is one direction of audio (inbound or outbound).
type leg struct {
	mu      sync.Mutex
	wav     *wavWriter
	pt      RTPPayloadType
	codec   string
	file    string
	packets uint64
}

func (l *leg) writeRTP(pkt []byte) {
	if l == nil || l.wav == nil || len(pkt) < 12 {
		return
	}
	// Parse RTP header to extract payload.
	cc := int(pkt[0] & 0x0F)
	hdrLen := 12 + 4*cc
	if hdrLen >= len(pkt) {
		return
	}
	pt := RTPPayloadType(pkt[1] & 0x7F)
	payload := pkt[hdrLen:]

	l.mu.Lock()
	defer l.mu.Unlock()
	l.packets++
	var err error
	switch pt {
	case PayloadPCMU:
		err = l.wav.WritePCMU(payload)
	case PayloadPCMA:
		err = l.wav.WritePCMA(payload)
	default:
		// Unsupported payload type — skip silently.
		return
	}
	if err != nil {
		log.Printf("capture: wav write error leg=%s err=%v", l.file, err)
	}
}

func (l *leg) close() recordingFile {
	if l == nil {
		return recordingFile{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	rec := recordingFile{
		File:         filepath.Base(l.file),
		Leg:          "",
		Codec:        l.codec,
		SampleRateHz: wavSampleRate,
		Channels:     wavChannels,
	}
	if l.wav != nil {
		rec.DurationSeconds = l.wav.DurationSeconds()
		rec.BytesWritten = l.wav.BytesWritten()
		if err := l.wav.Close(); err != nil {
			log.Printf("capture: wav close error leg=%s err=%v", l.file, err)
		}
		l.wav = nil
	}
	return rec
}

// CallCapture captures both legs of a call to WAV files and writes a JSON
// metadata file when closed.  All methods are safe for concurrent use.
type CallCapture struct {
	dir       string
	meta      CallMeta
	startedAt time.Time
	prefix    string

	inbound  *leg
	outbound *leg
}

// New creates a CallCapture.  dir is the root capture directory; it must
// already exist or be creatable.  meta provides call identity for the JSON.
// Returns nil (not an error) if dir is empty — callers can check for nil
// before calling WriteInboundRTP / WriteOutboundRTP.
func New(dir string, meta CallMeta, inboundPT, outboundPT RTPPayloadType) (*CallCapture, error) {
	if dir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("capture dir: %w", err)
	}

	now := time.Now().UTC()
	ts := now.Format("2006-01-02T15-04-05")
	key := meta.SessionKey
	if len(key) > 8 {
		key = key[:8]
	}
	prefix := fmt.Sprintf("%s_%s", ts, key)

	c := &CallCapture{
		dir:       dir,
		meta:      meta,
		startedAt: now,
		prefix:    prefix,
	}

	if ib, err := c.openLeg("inbound", inboundPT); err == nil {
		c.inbound = ib
	} else {
		log.Printf("capture: open inbound leg err=%v", err)
	}
	if ob, err := c.openLeg("outbound", outboundPT); err == nil {
		c.outbound = ob
	} else {
		log.Printf("capture: open outbound leg err=%v", err)
	}

	return c, nil
}

func (c *CallCapture) openLeg(role string, pt RTPPayloadType) (*leg, error) {
	codec := "PCMU"
	if pt == PayloadPCMA {
		codec = "PCMA"
	}
	name := fmt.Sprintf("%s_%s.wav", c.prefix, role)
	path := filepath.Join(c.dir, name)
	w, err := newWAVWriter(path)
	if err != nil {
		return nil, err
	}
	return &leg{wav: w, pt: pt, codec: codec, file: path}, nil
}

// WriteInboundRTP writes one raw RTP packet from the inbound (caller) leg.
func (c *CallCapture) WriteInboundRTP(pkt []byte) {
	if c == nil {
		return
	}
	c.inbound.writeRTP(pkt)
}

// WriteOutboundRTP writes one raw RTP packet from the outbound (peer) leg.
func (c *CallCapture) WriteOutboundRTP(pkt []byte) {
	if c == nil {
		return
	}
	c.outbound.writeRTP(pkt)
}

// Close finalises both WAV files and writes the metadata JSON.
func (c *CallCapture) Close() {
	if c == nil {
		return
	}

	endedAt := time.Now().UTC()
	duration := endedAt.Sub(c.startedAt).Seconds()

	ibRec := c.inbound.close()
	ibRec.Leg = "inbound"
	obRec := c.outbound.close()
	obRec.Leg = "outbound"

	var recs []recordingFile
	if ibRec.File != "" {
		recs = append(recs, ibRec)
	}
	if obRec.File != "" {
		recs = append(recs, obRec)
	}

	record := callRecord{
		CallMeta:        c.meta,
		StartedAt:       c.startedAt.Format(time.RFC3339Nano),
		EndedAt:         endedAt.Format(time.RFC3339Nano),
		DurationSeconds: duration,
		Recordings:      recs,
	}

	// Parse the binary sequence number and timestamp from the first packet's
	// RTP header — not available here, so we rely on WAV duration instead.
	// Full timestamp fidelity is available from the WAV files themselves.

	jsonPath := filepath.Join(c.dir, fmt.Sprintf("%s_meta.json", c.prefix))
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		log.Printf("capture: json marshal err=%v", err)
		return
	}
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		log.Printf("capture: write meta json err=%v", err)
	}
}

// RTPHeaderPayloadType extracts the payload type from a raw RTP packet.
// Returns 0xFF if the packet is too short.
func RTPHeaderPayloadType(pkt []byte) RTPPayloadType {
	if len(pkt) < 2 {
		return 0xFF
	}
	return RTPPayloadType(pkt[1] & 0x7F)
}

// RTPHeaderSeq extracts the sequence number from a raw RTP packet.
func RTPHeaderSeq(pkt []byte) uint16 {
	if len(pkt) < 4 {
		return 0
	}
	return binary.BigEndian.Uint16(pkt[2:4])
}

// RTPHeaderTimestamp extracts the RTP timestamp from a raw RTP packet.
func RTPHeaderTimestamp(pkt []byte) uint32 {
	if len(pkt) < 8 {
		return 0
	}
	return binary.BigEndian.Uint32(pkt[4:8])
}
