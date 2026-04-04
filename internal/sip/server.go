package sip

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"sipbridge/internal/config"
)

type Server struct {
	cfg config.SIPConfig
	router *Router

	cluster config.ClusterLimits

	tlsClient *tls.Config

	sessionMu sync.Mutex
	sessions  map[string]*fanoutSession
	legsByCallID map[string]*fanoutLeg
	ivrConfSessions map[string]*ivrConferenceFanoutSession
	bridgeCalls map[string]map[string]*bridgeCall
	ivrSessions map[string]*ivrSession

	// siprecRecordings maps logical session keys (see siprec_ctrl.go) to outbound SIPREC INVITE legs.
	siprecRecordings map[string]*fanoutLeg

	mu           sync.RWMutex
	started      bool
	udpConn      *net.UDPConn
	packetsRx    uint64
	bytesRx      uint64
	lastPacketAt time.Time
}

func (s *Server) ivrStartConferenceGroupFanout(ivrKey string, sess *ivrSession) {
	if s == nil || sess == nil || s.router == nil {
		log.Printf("IVR conferenceGroup fanout: not ready key=%s", ivrKey)
		return
	}
	groupID := strings.TrimSpace(sess.conferenceGroupID)
	if groupID == "" {
		log.Printf("IVR conferenceGroup fanout: missing group_id key=%s", ivrKey)
		return
	}

	cfg := s.router.CurrentConfig()
	g, ok := conferenceGroupByID(cfg, groupID)
	if !ok {
		log.Printf("IVR conferenceGroup fanout: group not found key=%s group_id=%s", ivrKey, groupID)
		return
	}

	callerSide := strings.TrimSpace(sess.conferenceCallerSide)
	if callerSide == "" {
		callerSide = "A"
	}
	log.Printf("IVR conferenceGroup fanout: start key=%s group_id=%s caller_side=%s", ivrKey, groupID, callerSide)
	var ring []config.Endpoint
	if callerSide == "A" {
		ring = append([]config.Endpoint(nil), g.SideB...)
	} else {
		ring = append([]config.Endpoint(nil), g.SideA...)
	}
	if len(ring) == 0 {
		log.Printf("IVR conferenceGroup fanout: no endpoints to ring group_id=%s caller_side=%s", groupID, callerSide)
		return
	}
	pref := strings.TrimSpace(sess.preferredRegion)
	if pref == "" {
		fromU := ExtractUserFromURI(ExtractURIFromAddressHeader(sess.fromHeader))
		pref = preferredRegionForConferenceCaller(cfg, g, fromU)
	}
	sortEndpointsByPreferredRegion(ring, pref)
	if pref != "" {
		log.Printf("IVR conferenceGroup fanout: region preference=%q group_id=%s", pref, groupID)
	}
	log.Printf("IVR conferenceGroup fanout: endpoints=%d group_id=%s", len(ring), groupID)

	ringTimeout := 30 * time.Second
	if g.RingTimeoutSeconds > 0 {
		ringTimeout = time.Duration(g.RingTimeoutSeconds) * time.Second
	}
	winnerKeep := time.Duration(0)
	if g.WinnerKeepRingingSeconds > 0 {
		winnerKeep = time.Duration(g.WinnerKeepRingingSeconds) * time.Second
	}

	conn := s.udpConn
	if conn == nil {
		log.Printf("IVR conferenceGroup fanout: UDP conn not ready")
		return
	}
	la := conn.LocalAddr().(*net.UDPAddr)
	localIP := advertisedIP(conn, sess.remote)
	localPort := la.Port

	fanKey := "ivrconf|" + ivrKey

	s.sessionMu.Lock()
	fs := s.ivrConfSessions[fanKey]
	if fs == nil {
		fs = &ivrConferenceFanoutSession{key: fanKey, ivrKey: ivrKey, groupID: groupID, callerSide: callerSide, createdAt: time.Now().UTC(), ringTimeout: ringTimeout, winnerKeepRinging: winnerKeep}
		s.ivrConfSessions[fanKey] = fs
	}
	fs.inboundBC = sipBackchannel{UDP: conn, Peer: sess.remote}
	s.sessionMu.Unlock()

	s.enforceIVRConfRingTimeout(fanKey, ringTimeout)

	for _, ep := range ring {
		if strings.TrimSpace(ep.SIPURI) == "" {
			continue
		}
		log.Printf("IVR conferenceGroup fanout: processing endpoint id=%s target=%s", strings.TrimSpace(ep.ID), strings.TrimSpace(ep.SIPURI))
		// Allocate RTP port for this outbound leg and offer active media.
		pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
		if err != nil {
			log.Printf("IVR conferenceGroup fanout: RTP listen failed target=%s err=%v", strings.TrimSpace(ep.SIPURI), err)
			continue
		}
		rtpLeg := newRTPSession(pc, nil, 0, 0, nil)
		rtpPort := rtpLeg.LocalPort()
		// Send/recv PCMU with no telephone-event; keep it simple for now.
		sdp := buildActivePCMUOffer(localIP, rtpPort)
		leg, err := s.emitOutboundInvite(fanKey, ep.SIPURI, sdp, rtpLeg, localIP, localPort)
		if err != nil {
			rtpLeg.Close()
			log.Printf("IVR conferenceGroup fanout: outbound INVITE failed target=%s err=%v", strings.TrimSpace(ep.SIPURI), err)
			continue
		}
		log.Printf("IVR conferenceGroup fanout invite target=%s call_id=%s", leg.targetURI, leg.callID)

		s.sessionMu.Lock()
		fs.legs = append(fs.legs, leg)
		s.legsByCallID[leg.callID] = leg
		s.sessionMu.Unlock()
	}
}

func buildActivePCMUOffer(ip string, rtpPort int) string {
	// Minimal active SDP for PCMU.
	family := "IP4"
	if parsed := net.ParseIP(strings.TrimSpace(ip)); parsed != nil {
		if parsed.To4() == nil {
			family = "IP6"
		}
	}
	var b strings.Builder
	b.WriteString("v=0\r\n")
	fmt.Fprintf(&b, "o=sipbridge 0 0 IN %s %s\r\n", family, ip)
	b.WriteString("s=SIPBridge\r\n")
	fmt.Fprintf(&b, "c=IN %s %s\r\n", family, ip)
	b.WriteString("t=0 0\r\n")
	fmt.Fprintf(&b, "m=audio %d RTP/AVP 0\r\n", rtpPort)
	b.WriteString("a=rtpmap:0 PCMU/8000\r\n")
	b.WriteString("a=sendrecv\r\n")
	return b.String()
}

type fanoutSession struct {
	key           string
	inboundInvite *Message
	inboundRemote *net.UDPAddr
	inboundBC     sipBackchannel
	createdAt     time.Time

	targetKind InviteTargetKind
	bridgeID   string
	// ConferenceGroupID is set when targetKind is InviteTargetKindConferenceGroup (direct SIP INVITE fanout).
	ConferenceGroupID string
	// ConferenceARD is true when the conference group type is ARD (join without re-ringing when line is up).
	ConferenceARD bool

	ringTimeout time.Duration

	legs []*fanoutLeg

	winnerCallID string
	terminated   bool
}

type fanoutLeg struct {
	sessionKey string
	targetURI  string

	callID  string
	branch  string
	fromTag string
	toHeader string
	// siprecByeURI is the Request-URI for BYE (Contact from 200 OK) for SIPREC legs.
	siprecByeURI string

	// Outbound TLS (or TCP) to SBC; UDP legs use nil and send via udpConn + outboundDest.
	outboundConn net.Conn
	viaTransport SIPViaTransport
	localViaHost string
	localViaPort int

	rtp *rtpSession

	finalStatus int
}

type bridgeCall struct {
	bridgeID string

	callID  string
	fromTag string
	toTag   string

	fromHeader string
	toHeader   string

	contactURI string

	remote *net.UDPAddr
	createdAt time.Time

	// Set when the leg joined via IVR dial-in authorization (participant PIN).
	userID          string
	userDisplayName string
	pinLen          int

	rtp *rtpSession
}

type Stats struct {
	Started      bool      `json:"started"`
	PacketsRx    uint64    `json:"packets_rx"`
	BytesRx      uint64    `json:"bytes_rx"`
	LastPacketAt time.Time `json:"last_packet_at"`
}

func NewServer(cfg config.SIPConfig, router *Router, cluster config.ClusterLimits) *Server {
	return &Server{
		cfg:         cfg,
		router:      router,
		cluster:     cluster,
		sessions:    make(map[string]*fanoutSession),
		legsByCallID: make(map[string]*fanoutLeg),
		ivrConfSessions: make(map[string]*ivrConferenceFanoutSession),
		bridgeCalls: make(map[string]map[string]*bridgeCall),
		ivrSessions: make(map[string]*ivrSession),
		siprecRecordings: make(map[string]*fanoutLeg),
	}
}

// ActiveDialogCount is an approximate count of concurrent SIP dialogs (bridge legs, IVR, fanout).
func (s *Server) ActiveDialogCount() int {
	if s == nil {
		return 0
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	n := len(s.sessions) + len(s.ivrSessions) + len(s.ivrConfSessions)
	for _, m := range s.bridgeCalls {
		n += len(m)
	}
	return n
}

// ClusterLimits returns a copy of admission-control settings.
func (s *Server) ClusterLimits() config.ClusterLimits {
	if s == nil {
		return config.ClusterLimits{}
	}
	return s.cluster
}

// CapacitySnapshot is used by GET /v1/capacity for load balancers and peer aggregation.
func (s *Server) CapacitySnapshot() map[string]any {
	if s == nil {
		return map[string]any{"active_dialogs": 0}
	}
	active := s.ActiveDialogCount()
	max := s.cluster.MaxConcurrentCalls
	soft := config.EffectiveSoftMax(s.cluster)
	var load float64
	if max > 0 {
		load = float64(active) / float64(max)
	}
	softLoad := 0.0
	if soft > 0 {
		softLoad = float64(active) / float64(soft)
	}
	hardFull := max > 0 && active >= max
	softHot := soft > 0 && active >= soft
	return map[string]any{
		"active_dialogs":            active,
		"max_concurrent_calls":      max,
		"soft_max_concurrent_calls": soft,
		"load_ratio":                load,
		"soft_load_ratio":           softLoad,
		"accepting_new_calls":       !hardFull,
		"soft_overloaded":           softHot,
		"hard_overloaded":           hardFull,
	}
}

func (s *Server) tryRejectInviteForCapacity(msg *Message, conn *net.UDPConn, remote *net.UDPAddr) bool {
	if s == nil || s.cluster.MaxConcurrentCalls <= 0 {
		return false
	}
	active := s.ActiveDialogCount()
	if active < s.cluster.MaxConcurrentCalls {
		return false
	}
	if s.cluster.OverflowRedirectEnabled && strings.TrimSpace(s.cluster.OverflowRedirectSIPURI) != "" {
		uri := strings.TrimSpace(s.cluster.OverflowRedirectSIPURI)
		extra := map[string]string{
			"Contact": fmt.Sprintf("<%s>", uri),
		}
		resp, _ := BuildResponse(msg, 302, "", extra, nil)
		if _, err := conn.WriteToUDP(resp, remote); err != nil {
			log.Printf("SIP capacity: 302 write err=%v", err)
		} else {
			log.Printf("SIP capacity: 302 redirect active=%d max=%d contact=%s", active, s.cluster.MaxConcurrentCalls, uri)
		}
		return true
	}
	extra := map[string]string{
		"Retry-After": "5",
	}
	resp, _ := BuildResponse(msg, 503, "Service Unavailable", extra, nil)
	if _, err := conn.WriteToUDP(resp, remote); err != nil {
		log.Printf("SIP capacity: 503 write err=%v", err)
	} else {
		log.Printf("SIP capacity: 503 overload active=%d max=%d", active, s.cluster.MaxConcurrentCalls)
	}
	return true
}

type ivrState string

const (
	ivrStateCollectBridge ivrState = "collect_bridge"
	ivrStateCollectUser   ivrState = "collect_user"
	ivrStateJoined        ivrState = "joined"
	ivrStateJoinedConferenceGroup ivrState = "joined_conference_group"
)

type ivrSession struct {
	key string

	remote *net.UDPAddr
	fromHeader string
	toHeader   string
	contactURI string
	createdAt time.Time

	state ivrState
	buf   strings.Builder
	bridgeAccess string
	participantID string
	conferenceGroupID string
	conferenceGroupType string
	conferenceCallerSide string
	// preferredRegion is set after IVR PIN auth from User.region; used to order outbound ring targets.
	preferredRegion string

	rtp *rtpSession
	dtmfViaRTP bool
}

type ivrConferenceFanoutSession struct {
	key string
	ivrKey string
	groupID string
	callerSide string
	inboundBC sipBackchannel
	createdAt time.Time
	ringTimeout time.Duration
	winnerKeepRinging time.Duration
	legs []*fanoutLeg
	winnerCallID string
	terminated bool
	rtp *rtpSession
}

func (s *Server) enforceIVRConfRingTimeout(fanKey string, d time.Duration) {
	if d <= 0 {
		return
	}
	go func() {
		t := time.NewTimer(d)
		defer t.Stop()
		<-t.C
		s.sessionMu.Lock()
		fs := s.ivrConfSessions[fanKey]
		if fs == nil || fs.terminated || fs.winnerCallID != "" {
			s.sessionMu.Unlock()
			return
		}
		fs.terminated = true
		legs := append([]*fanoutLeg(nil), fs.legs...)
		s.sessionMu.Unlock()

		for _, l := range legs {
			li, lp := s.localViaForLeg(l)
			cancel := BuildOutboundCancel(l.targetURI, li, lp, l.callID, l.branch, l.fromTag, l.viaTransport)
			_ = s.writeLegOutbound(l, cancel)
			if l.rtp != nil {
				l.rtp.Close()
			}
		}
		log.Printf("IVR conferenceGroup fanout timeout: canceled all legs fan_key=%s", fanKey)
	}()
}

type BridgeCallInfo struct {
	BridgeID   string    `json:"bridge_id"`
	CallID     string    `json:"call_id"`
	FromTag    string    `json:"from_tag"`
	ToTag      string    `json:"to_tag"`
	FromURI    string    `json:"from_uri"`
	ToURI      string    `json:"to_uri"`
	ContactURI string    `json:"contact_uri"`
	RemoteAddr string    `json:"remote_addr"`
	CreatedAt  time.Time `json:"created_at"`
	// Populated for IVR dial-in legs (credentials used for MI / attendance).
	// UserID is the config user id (e.g. bank employee id); EmployeeID duplicates it for API clarity.
	UserID      string `json:"user_id,omitempty"`
	EmployeeID  string `json:"employee_id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	PinMasked   string `json:"pin_masked,omitempty"`
}

// MIAttendanceRow is one active bridge leg with optional dial-in identity for MI dashboards.
type MIAttendanceRow struct {
	BridgeID    string    `json:"bridge_id"`
	CallID      string    `json:"call_id"`
	UserID      string    `json:"user_id,omitempty"`
	EmployeeID  string    `json:"employee_id,omitempty"`
	DisplayName string    `json:"display_name,omitempty"`
	PinMasked   string    `json:"pin_masked,omitempty"`
	RemoteAddr  string    `json:"remote_addr"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Server) ListBridgeCalls(bridgeID string) []BridgeCallInfo {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()

	m := s.bridgeCalls[bridgeID]
	if len(m) == 0 {
		return nil
	}
	res := make([]BridgeCallInfo, 0, len(m))
	for _, c := range m {
		remote := ""
		if c.remote != nil {
			remote = c.remote.String()
		}
		pinMasked := ""
		if c.pinLen > 0 {
			pinMasked = MaskPINDigits(c.pinLen)
		}
		empID := strings.TrimSpace(c.userID)
		res = append(res, BridgeCallInfo{
			BridgeID:    c.bridgeID,
			CallID:      c.callID,
			FromTag:     c.fromTag,
			ToTag:       c.toTag,
			FromURI:     ExtractURIFromAddressHeader(c.fromHeader),
			ToURI:       ExtractURIFromAddressHeader(c.toHeader),
			ContactURI:  c.contactURI,
			RemoteAddr:  remote,
			CreatedAt:   c.createdAt,
			UserID:      empID,
			EmployeeID:  empID,
			DisplayName: c.userDisplayName,
			PinMasked:   pinMasked,
		})
	}
	return res
}

// ConferenceGroupLiveSession is one in-memory conference line group session (IVR dial-in or direct INVITE fanout).
type ConferenceGroupLiveSession struct {
	GroupID             string    `json:"group_id"`
	Source              string    `json:"source"` // ivr | direct_invite
	Phase               string    `json:"phase"`  // ivr_joined | fanout_ringing | media_connected | direct_fanout | direct_connected
	SessionRef          string    `json:"session_ref"`
	CallerSide          string    `json:"caller_side,omitempty"`
	ConferenceGroupType string    `json:"conference_group_type,omitempty"`
	FanoutLegs          int       `json:"fanout_legs"`
	WinnerEstablished   bool      `json:"winner_established"`
	CreatedAt           time.Time `json:"created_at"`
	RemoteAddr          string    `json:"remote_addr,omitempty"`
	PreferredRegion     string    `json:"preferred_region,omitempty"`
}

// ListConferenceGroupUsage returns active conference group sessions for operations dashboards.
func (s *Server) ListConferenceGroupUsage() []ConferenceGroupLiveSession {
	if s == nil {
		return nil
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()

	var out []ConferenceGroupLiveSession
	seenIVR := make(map[string]struct{})

	for ivrKey, sess := range s.ivrSessions {
		if sess.state != ivrStateJoinedConferenceGroup {
			continue
		}
		gid := strings.TrimSpace(sess.conferenceGroupID)
		if gid == "" {
			continue
		}
		fanKey := "ivrconf|" + ivrKey
		fs := s.ivrConfSessions[fanKey]
		remote := ""
		if sess.remote != nil {
			remote = sess.remote.String()
		}
		phase := "ivr_joined"
		legs := 0
		winner := false
		if fs != nil && !fs.terminated {
			legs = len(fs.legs)
			winner = fs.winnerCallID != ""
			if !winner {
				phase = "fanout_ringing"
			} else {
				phase = "media_connected"
			}
		}
		out = append(out, ConferenceGroupLiveSession{
			GroupID:             gid,
			Source:              "ivr",
			Phase:               phase,
			SessionRef:          ivrKey,
			CallerSide:          sess.conferenceCallerSide,
			ConferenceGroupType: sess.conferenceGroupType,
			FanoutLegs:          legs,
			WinnerEstablished:   winner,
			CreatedAt:           sess.createdAt,
			RemoteAddr:          remote,
			PreferredRegion:     sess.preferredRegion,
		})
		seenIVR[ivrKey] = struct{}{}
	}

	for fanKey, fs := range s.ivrConfSessions {
		if fs.terminated {
			continue
		}
		ivrKey := strings.TrimPrefix(fanKey, "ivrconf|")
		if _, ok := seenIVR[ivrKey]; ok {
			continue
		}
		gid := strings.TrimSpace(fs.groupID)
		phase := "fanout_ringing"
		if fs.winnerCallID != "" {
			phase = "media_connected"
		}
		out = append(out, ConferenceGroupLiveSession{
			GroupID:           gid,
			Source:            "ivr",
			Phase:             phase,
			SessionRef:        ivrKey,
			CallerSide:        fs.callerSide,
			FanoutLegs:        len(fs.legs),
			WinnerEstablished: fs.winnerCallID != "",
			CreatedAt:         fs.createdAt,
		})
	}

	for sessKey, sess := range s.sessions {
		if sess.targetKind != InviteTargetKindConferenceGroup || sess.terminated {
			continue
		}
		gid := strings.TrimSpace(sess.ConferenceGroupID)
		if gid == "" {
			continue
		}
		remote := ""
		if sess.inboundRemote != nil {
			remote = sess.inboundRemote.String()
		}
		phase := "direct_fanout"
		if sess.winnerCallID != "" {
			phase = "direct_connected"
		}
		out = append(out, ConferenceGroupLiveSession{
			GroupID:           gid,
			Source:            "direct_invite",
			Phase:             phase,
			SessionRef:        sessKey,
			FanoutLegs:        len(sess.legs),
			WinnerEstablished: sess.winnerCallID != "",
			CreatedAt:         sess.createdAt,
			RemoteAddr:        remote,
		})
	}

	return out
}

// ListMIAttendance returns all active bridge legs with optional IVR dial-in identity.
func (s *Server) ListMIAttendance() []MIAttendanceRow {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	var out []MIAttendanceRow
	for bid, m := range s.bridgeCalls {
		for _, c := range m {
			remote := ""
			if c.remote != nil {
				remote = c.remote.String()
			}
			pinMasked := ""
			if c.pinLen > 0 {
				pinMasked = MaskPINDigits(c.pinLen)
			}
			empID := strings.TrimSpace(c.userID)
			out = append(out, MIAttendanceRow{
				BridgeID:    bid,
				CallID:      c.callID,
				UserID:      empID,
				EmployeeID:  empID,
				DisplayName: c.userDisplayName,
				PinMasked:   pinMasked,
				RemoteAddr:  remote,
				CreatedAt:   c.createdAt,
			})
		}
	}
	return out
}

// ResetBridge sends BYE to all active legs on a bridge (disconnect everyone).
func (s *Server) ResetBridge(bridgeID string) error {
	calls := s.ListBridgeCalls(bridgeID)
	var last error
	for _, c := range calls {
		if err := s.DropBridgeCall(bridgeID, c.CallID, c.FromTag); err != nil {
			last = err
		}
	}
	return last
}

func (s *Server) DropBridgeCall(bridgeID, callID, fromTag string) error {
	key := callID + "|" + fromTag

	s.sessionMu.Lock()
	byBridge := s.bridgeCalls[bridgeID]
	if byBridge == nil {
		s.sessionMu.Unlock()
		return fmt.Errorf("bridge call not found")
	}
	c, ok := byBridge[key]
	if !ok {
		s.sessionMu.Unlock()
		return fmt.Errorf("bridge call not found")
	}
	remote := c.remote
	contactURI := c.contactURI
	fromHeader := c.fromHeader
	toHeader := c.toHeader
	s.sessionMu.Unlock()

	s.mu.RLock()
	conn := s.udpConn
	s.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("sip server not started")
	}
	if remote == nil {
		return fmt.Errorf("missing remote addr")
	}
	if strings.TrimSpace(contactURI) == "" {
		return fmt.Errorf("missing contact uri")
	}

	la := conn.LocalAddr().(*net.UDPAddr)
	bye := buildInboundDialogBye(contactURI, la.IP.String(), la.Port, callID, fromHeader, toHeader)
	if _, err := conn.WriteToUDP(bye, remote); err != nil {
		return fmt.Errorf("send bye: %w", err)
	}

	s.sessionMu.Lock()
	byBridge = s.bridgeCalls[bridgeID]
	if byBridge != nil {
		if c2, ok := byBridge[key]; ok && c2 != nil && c2.rtp != nil {
			c2.rtp.Close()
			c2.rtp = nil
		}
		delete(byBridge, key)
		if len(byBridge) == 0 {
			delete(s.bridgeCalls, bridgeID)
		}
	}
	s.sessionMu.Unlock()
	s.stopSIPRECRecording("bridge:" + bridgeID + ":" + key)
	return nil
}

func ensureTLSClient(cfg config.SIPConfig) (*tls.Config, error) {
	if cfg.OutboundTransport != "tls" {
		return nil, nil
	}
	return NewTLSClientConfig(cfg)
}

func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	tc, err := ensureTLSClient(s.cfg)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("TLS outbound: %w", err)
	}
	s.tlsClient = tc
	addr := &net.UDPAddr{IP: net.ParseIP(s.cfg.BindAddr), Port: s.cfg.UDPPort}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("listen udp: %w", err)
	}
	s.udpConn = conn
	s.started = true
	s.mu.Unlock()

	log.Printf("SIP UDP listening on %s", conn.LocalAddr().String())

	buf := make([]byte, 64*1024)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remote, err := conn.ReadFromUDP(buf)
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		if err != nil {
			nErr, ok := err.(net.Error)
			if ok && nErr.Timeout() {
				continue
			}
			return fmt.Errorf("udp read: %w", err)
		}

		s.mu.Lock()
		s.packetsRx++
		s.bytesRx += uint64(n)
		s.lastPacketAt = time.Now().UTC()
		s.mu.Unlock()

		payload := append([]byte(nil), buf[:n]...)
		snippet := string(payload[:min(n, 1200)])
		log.Printf("SIP UDP rx local=%s from=%s bytes=%d\n%s", conn.LocalAddr().String(), remote.String(), n, snippet)

		msg, pErr := ParseMessage(payload)
		if pErr != nil {
			log.Printf("SIP parse error from=%s err=%v", remote.String(), pErr)
			continue
		}
		if !msg.IsRequest {
			s.handleResponse(msg)
			continue
		}

		switch msg.Method {
		case "OPTIONS":
			extra := map[string]string{
				"Allow":     "INVITE, ACK, BYE, CANCEL, OPTIONS, INFO, REFER, NOTIFY, SUBSCRIBE",
				"Supported": "replaces, norefersub",
			}
			resp, _ := BuildResponse(msg, 200, "OK", extra, nil)
			if _, wErr := conn.WriteToUDP(resp, remote); wErr != nil {
				log.Printf("SIP UDP tx error method=OPTIONS to=%s err=%v", remote.String(), wErr)
			}
		case "REGISTER":
			// SIPBridge currently doesn't act as a registrar, but many softphones will attempt to REGISTER.
			// Return 200 OK so the client can proceed with calls in lab setups.
			extra := map[string]string{
				"Allow":  "INVITE, ACK, BYE, CANCEL, OPTIONS, INFO, REFER, REGISTER",
				"Expires": "0",
			}
			resp, _ := BuildResponse(msg, 200, "OK", extra, nil)
			if _, wErr := conn.WriteToUDP(resp, remote); wErr != nil {
				log.Printf("SIP UDP tx error method=REGISTER status=200 to=%s err=%v", remote.String(), wErr)
			} else {
				log.Printf("SIP UDP tx method=REGISTER status=200 to=%s", remote.String())
			}
		case "INVITE":
			trying, _ := BuildResponse(msg, 100, "Trying", nil, nil)
			if _, wErr := conn.WriteToUDP(trying, remote); wErr != nil {
				log.Printf("SIP UDP tx error method=INVITE status=100 to=%s err=%v", remote.String(), wErr)
			} else {
				log.Printf("SIP UDP tx method=INVITE status=100 to=%s", remote.String())
			}

			localIP := conn.LocalAddr().(*net.UDPAddr).IP.String()
			localPort := conn.LocalAddr().(*net.UDPAddr).Port
			advertiseIP := advertisedIP(conn, remote)
			fromHdr := msg.Header("from")
			toHdr := msg.Header("to")
			contactURI := ExtractURIFromAddressHeader(msg.Header("contact"))
			fromURI := ExtractURIFromAddressHeader(fromHdr)
			fromUser := ExtractUserFromURI(fromURI)
			fromTag := extractTag(fromHdr)
			callID := strings.TrimSpace(msg.Header("call-id"))
			if callID == "" || fromTag == "" {
				bad, _ := BuildResponse(msg, 400, "Bad Request", nil, nil)
				if _, wErr := conn.WriteToUDP(bad, remote); wErr != nil {
					log.Printf("SIP UDP tx error method=INVITE status=400 to=%s err=%v", remote.String(), wErr)
				} else {
					log.Printf("SIP UDP tx method=INVITE status=400 to=%s", remote.String())
				}
				break
			}
			sessKey := callID + "|" + fromTag
			toTag := extractTag(toHdr)

			if toTag != "" {
				if s.tryHandleBridgeReinvite(msg, conn, remote, advertiseIP, localPort, sessKey, toHdr) {
					break
				}
				if s.tryHandleIVRReinvite(msg, conn, remote, advertiseIP, localPort, sessKey, toHdr) {
					break
				}
				un481, _ := BuildResponse(msg, 481, "Call/Transaction Does Not Exist", nil, nil)
				if _, wErr := conn.WriteToUDP(un481, remote); wErr != nil {
					log.Printf("SIP UDP tx error method=INVITE status=481 to=%s err=%v", remote.String(), wErr)
				} else {
					log.Printf("SIP UDP tx method=INVITE status=481 to=%s", remote.String())
				}
				break
			}

			if toTag == "" {
				if s.tryRejectInviteForCapacity(msg, conn, remote) {
					break
				}
			}

			if s.router == nil {
				unavail, _ := BuildResponse(msg, 503, "Service Unavailable", nil, nil)
				if _, wErr := conn.WriteToUDP(unavail, remote); wErr != nil {
					log.Printf("SIP UDP tx error method=INVITE status=503 to=%s err=%v", remote.String(), wErr)
				} else {
					log.Printf("SIP UDP tx method=INVITE status=503 to=%s", remote.String())
				}
				break
			}
			target, ok := s.router.MatchInvite(msg)
			if !ok {
				notFound, _ := BuildResponse(msg, 404, "Not Found", nil, nil)
				if _, wErr := conn.WriteToUDP(notFound, remote); wErr != nil {
					log.Printf("SIP UDP tx error method=INVITE status=404 to=%s err=%v", remote.String(), wErr)
				} else {
					log.Printf("SIP UDP tx method=INVITE status=404 to=%s", remote.String())
				}
				break
			}

			if target.Kind == InviteTargetKindIVR {
				// IVR entrypoint: answer and collect digits via SIP INFO. Start RTP so we can play prompts.
				toWithTag, _ := ensureToTagWithValue(toHdr)

				offer, okOffer := parseSDPAudioOffer(string(msg.Body), remote.IP)
				var rtpSess *rtpSession
				rtpPort := 0
				if okOffer && offer.HasPCMU {
					pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
					if err == nil {
						telPT := uint8(0)
						if offer.TelephoneEventPT > 0 && offer.TelephoneEventPT < 128 {
							telPT = uint8(offer.TelephoneEventPT)
						}
						remoteRTPIP := offer.Addr
						if remote != nil && remote.IP != nil && !remote.IP.IsUnspecified() {
							remoteRTPIP = remote.IP
						}
						rtpSess = newRTPSession(pc, &net.UDPAddr{IP: remoteRTPIP, Port: offer.Port}, 0, telPT, func(d string) {
							parts := strings.SplitN(sessKey, "|", 2)
							callID := ""
							fromTag := ""
							if len(parts) == 2 {
								callID = parts[0]
								fromTag = parts[1]
							}
							s.sessionMu.Lock()
							sess := s.ivrSessions[sessKey]
							if sess != nil {
								s.handleIVRDigitLocked(sessKey, sess, d, callID, fromTag)
							}
							s.sessionMu.Unlock()
						})
						rtpSess.StartReceiver()
						rtpPort = rtpSess.LocalPort()
					}
				}

				if rtpPort == 0 {
					rtpPort = localPort
				}
				sdp := buildIVRSDPAnswer(advertiseIP, rtpPort, offer.TelephoneEventPT)
				if rtpSess != nil {
					log.Printf("IVR RTP negotiated local_rtp_port=%d remote_rtp=%s telephone_event_pt=%d", rtpPort, rtpSess.remote.String(), offer.TelephoneEventPT)
				} else {
					log.Printf("IVR RTP not negotiated (no PCMU offer?) remote=%s offered_tel_pt=%d", remote.String(), offer.TelephoneEventPT)
				}
				log.Printf("IVR SDP answer:\n%s", sdp)
				extra := map[string]string{
					"Contact":      fmt.Sprintf("<sip:sipbridge@%s;transport=udp>", net.JoinHostPort(advertiseIP, strconv.Itoa(localPort))),
					"Content-Type": "application/sdp",
					"Allow":        "INVITE, ACK, BYE, CANCEL, OPTIONS, INFO, REFER",
					"Supported":    "replaces, norefersub",
					"To":           toWithTag,
				}
				okResp, _ := BuildResponse(msg, 200, "OK", extra, []byte(sdp))
				_, _ = conn.WriteToUDP(okResp, remote)

				if rtpSess != nil {
					go rtpSess.PlayTone(880, 1200*time.Millisecond, 0.25)
				}

				s.sessionMu.Lock()
				s.ivrSessions[sessKey] = &ivrSession{
					key: sessKey,
					remote: remote,
					fromHeader: fromHdr,
					toHeader: toWithTag,
					contactURI: contactURI,
					createdAt: time.Now().UTC(),
					state: ivrStateCollectBridge,
					rtp: rtpSess,
					dtmfViaRTP: rtpSess != nil && offer.TelephoneEventPT > 0,
				}
				s.sessionMu.Unlock()
				break
			}

			var ring []config.Endpoint
			ringTimeout := 30 * time.Second
			ardJoinDone := false

			switch target.Kind {
			case InviteTargetKindConferenceGroup:
				callerSide := ""
				if fromUser != "" {
					for _, ep := range target.Group.SideA {
						if ExtractUserFromURI(ep.SIPURI) == fromUser {
							callerSide = "A"
							break
						}
					}
					if callerSide == "" {
						for _, ep := range target.Group.SideB {
							if ExtractUserFromURI(ep.SIPURI) == fromUser {
								callerSide = "B"
								break
							}
						}
					}
				}
				if callerSide == "A" {
					ring = append([]config.Endpoint(nil), target.Group.SideB...)
				} else if callerSide == "B" {
					ring = append([]config.Endpoint(nil), target.Group.SideA...)
				} else {
					ring = nil
				}
				if len(ring) > 0 {
					pref := preferredRegionForConferenceCaller(s.router.CurrentConfig(), target.Group, fromUser)
					sortEndpointsByPreferredRegion(ring, pref)
					if pref != "" {
						log.Printf("SIP INVITE conference ring order: region preference=%q group_id=%s", pref, target.Group.ID)
					}
				}
				if target.Group.RingTimeoutSeconds > 0 {
					ringTimeout = time.Duration(target.Group.RingTimeoutSeconds) * time.Second
				}
				// ARD: if the line already has participants, join without fanout or 180 ringing.
				if isARDGroup(target.Group) {
					gid := strings.TrimSpace(target.Group.ID)
					if gid != "" && s.ardEstablishedParticipantCount(gid) > 0 {
						s.sessionMu.Lock()
						dup := false
						if _, exists := s.sessions[sessKey]; exists {
							dup = true
						}
						if m := s.bridgeCalls[syntheticARDBridgeID(gid)]; m != nil {
							if _, exists := m[sessKey]; exists {
								dup = true
							}
						}
						s.sessionMu.Unlock()
						if dup {
							busy, _ := BuildResponse(msg, 486, "Busy Here", nil, nil)
							if _, wErr := conn.WriteToUDP(busy, remote); wErr != nil {
								log.Printf("SIP UDP tx error method=INVITE status=486 to=%s err=%v", remote.String(), wErr)
							}
						} else {
							s.answerARDJoinInbound(msg, conn, remote, advertiseIP, localPort, sessKey, gid, fromHdr, contactURI)
						}
						ardJoinDone = true
					}
				}
			case InviteTargetKindBridge:
				bridge := target.Bridge

				var callerPairID string
				var callerEnd string
				if fromUser != "" {
					for _, p := range bridge.Participants {
						if p.SIPURI == "" {
							continue
						}
						if ExtractUserFromURI(p.SIPURI) == fromUser {
							callerPairID = p.PairID
							callerEnd = strings.ToUpper(strings.TrimSpace(p.End))
							break
						}
					}
				}
				for _, p := range bridge.Participants {
					if p.SIPURI == "" {
						continue
					}
					if callerPairID != "" {
						if p.PairID != callerPairID {
							continue
						}
						pEnd := strings.ToUpper(strings.TrimSpace(p.End))
						if pEnd == "" || callerEnd == "" {
							continue
						}
						if pEnd == callerEnd {
							continue
						}
					} else {
						if fromUser != "" && ExtractUserFromURI(p.SIPURI) == fromUser {
							continue
						}
					}
					ring = append(ring, config.Endpoint{ID: p.ID, DisplayName: p.DisplayName, SIPURI: p.SIPURI, Location: p.Location})
				}
				if len(ring) > 0 {
					pref := preferredRegionForBridgeCaller(s.router.CurrentConfig(), target.Bridge, fromUser)
					sortEndpointsByPreferredRegion(ring, pref)
					if pref != "" {
						log.Printf("SIP INVITE bridge ring order: region preference=%q bridge_id=%s", pref, target.Bridge.ID)
					}
				}
			}

			if ardJoinDone {
				break
			}

			if len(ring) == 0 {
				log.Printf("SIP INVITE no ring targets call_id=%s from_user=%s target_kind=%v", callID, fromUser, target.Kind)
				if target.Kind == InviteTargetKindBridge {
					// Room-style behavior: allow joining a bridge even if no other participants are present.
					toWithTag, toTag := ensureToTagWithValue(toHdr)
					sdp := buildMinimalSDP(advertiseIP, localPort)
					extra := map[string]string{
						"Contact":      fmt.Sprintf("<sip:sipbridge@%s;transport=udp>", net.JoinHostPort(advertiseIP, strconv.Itoa(localPort))),
						"Content-Type": "application/sdp",
						"Allow":        "INVITE, ACK, BYE, CANCEL, OPTIONS, INFO, REFER",
						"Supported":    "replaces, norefersub",
						"To":           toWithTag,
					}
					okResp, _ := BuildResponse(msg, 200, "OK", extra, []byte(sdp))
					if _, wErr := conn.WriteToUDP(okResp, remote); wErr != nil {
						log.Printf("SIP UDP tx error method=INVITE status=200 to=%s err=%v", remote.String(), wErr)
					} else {
						log.Printf("SIP UDP tx method=INVITE status=200 to=%s", remote.String())
					}

					s.sessionMu.Lock()
					m := s.bridgeCalls[target.Bridge.ID]
					if m == nil {
						m = make(map[string]*bridgeCall)
						s.bridgeCalls[target.Bridge.ID] = m
					}
					m[sessKey] = &bridgeCall{
						bridgeID: target.Bridge.ID,
						callID: callID,
						fromTag: fromTag,
						toTag: toTag,
						fromHeader: fromHdr,
						toHeader: toWithTag,
						contactURI: contactURI,
						remote: remote,
						createdAt: time.Now().UTC(),
					}
					s.sessionMu.Unlock()
					s.tryStartSIPRECForBridge(target.Bridge.ID, sessKey)
					break
				}
				fail, _ := BuildResponse(msg, 480, "Temporarily Unavailable", nil, nil)
				if _, wErr := conn.WriteToUDP(fail, remote); wErr != nil {
					log.Printf("SIP UDP tx error method=INVITE status=480 to=%s err=%v", remote.String(), wErr)
				} else {
					log.Printf("SIP UDP tx method=INVITE status=480 to=%s", remote.String())
				}
				break
			}

			s.sessionMu.Lock()
			if _, exists := s.sessions[sessKey]; exists {
				s.sessionMu.Unlock()
				busy, _ := BuildResponse(msg, 486, "Busy Here", nil, nil)
				if _, wErr := conn.WriteToUDP(busy, remote); wErr != nil {
					log.Printf("SIP UDP tx error method=INVITE status=486 to=%s err=%v", remote.String(), wErr)
				}
				break
			}
			bridgeID := ""
			confGroupID := ""
			if target.Kind == InviteTargetKindBridge {
				bridgeID = target.Bridge.ID
			}
			if target.Kind == InviteTargetKindConferenceGroup {
				confGroupID = strings.TrimSpace(target.Group.ID)
			}
			sess := &fanoutSession{
				key:                sessKey,
				inboundInvite:      msg,
				inboundRemote:      remote,
				inboundBC:          sipBackchannel{UDP: conn, Peer: remote},
				createdAt:          time.Now().UTC(),
				targetKind:         target.Kind,
				bridgeID:           bridgeID,
				ConferenceGroupID:  confGroupID,
				ConferenceARD:      target.Kind == InviteTargetKindConferenceGroup && isARDGroup(target.Group),
				ringTimeout:        ringTimeout,
			}
			s.sessions[sessKey] = sess
			s.sessionMu.Unlock()

			for _, ep := range ring {
				if ep.SIPURI == "" {
					continue
				}
				sdp := buildMinimalSDP(localIP, localPort)
				leg, err := s.emitOutboundInvite(sessKey, ep.SIPURI, sdp, nil, localIP, localPort)
				if err != nil {
					log.Printf("SIP outbound leg failed target=%s err=%v", ep.SIPURI, err)
					continue
				}
				s.sessionMu.Lock()
				sess.legs = append(sess.legs, leg)
				s.legsByCallID[leg.callID] = leg
				s.sessionMu.Unlock()
				log.Printf("SIP outbound_invite target=%s call_id=%s", leg.targetURI, leg.callID)
			}

			ringing, _ := BuildResponse(msg, 180, "Ringing", nil, nil)
			if _, wErr := conn.WriteToUDP(ringing, remote); wErr != nil {
				log.Printf("SIP UDP tx error method=INVITE status=180 to=%s err=%v", remote.String(), wErr)
			} else {
				log.Printf("SIP UDP tx method=INVITE status=180 to=%s", remote.String())
			}

			go s.enforceRingTimeout(sessKey, ringTimeout)
		case "ACK":
			// No response to ACK.
			continue
		case "INFO":
			s.handleInboundInfo(msg, conn, remote)
		case "BYE":
			s.handleInboundBye(msg, conn, remote)
		case "CANCEL":
			s.handleInboundCancel(msg, conn, remote)
		default:
			extra := map[string]string{
				"Allow": "INVITE, ACK, BYE, CANCEL, OPTIONS, INFO, REFER",
			}
			resp, _ := BuildResponse(msg, 501, "Not Implemented", extra, nil)
			if _, wErr := conn.WriteToUDP(resp, remote); wErr != nil {
				log.Printf("SIP UDP tx error method=%s status=501 to=%s err=%v", msg.Method, remote.String(), wErr)
			}
		}
	}
}

func (s *Server) handleInboundInfo(msg *Message, conn *net.UDPConn, remote *net.UDPAddr) {
	okResp, _ := BuildResponse(msg, 200, "OK", nil, nil)
	_, _ = conn.WriteToUDP(okResp, remote)

	callID := strings.TrimSpace(msg.Header("call-id"))
	fromTag := extractTag(msg.Header("from"))
	if callID == "" || fromTag == "" {
		return
	}
	key := callID + "|" + fromTag

	s.sessionMu.Lock()
	sess, ok := s.ivrSessions[key]
	if !ok {
		s.sessionMu.Unlock()
		return
	}
	if sess.dtmfViaRTP {
		s.sessionMu.Unlock()
		return
	}

	// Very small SIP INFO DTMF parser. Expect application/dtmf-relay body like:
	// Signal=1\r\nDuration=160
	body := strings.TrimSpace(string(msg.Body))
	digit := ""
	for _, line := range strings.Split(body, "\n") {
		l := strings.TrimSpace(line)
		lower := strings.ToLower(l)
		if strings.HasPrefix(lower, "signal=") {
			digit = strings.TrimSpace(l[len("Signal="):])
			break
		}
		if strings.HasPrefix(lower, "signal:") {
			digit = strings.TrimSpace(l[len("Signal:"):])
			break
		}
	}
	if digit == "" {
		s.sessionMu.Unlock()
		return
	}
	// Feed into shared IVR digit handler.
	s.handleIVRDigitLocked(key, sess, digit, callID, fromTag)
	s.sessionMu.Unlock()
}

func (s *Server) handleIVRDigitLocked(key string, sess *ivrSession, digit string, callID string, fromTag string) {
	if sess == nil {
		return
	}
	log.Printf("IVR digit key=%s state=%s digit=%q", key, sess.state, digit)

	if sess.state == ivrStateJoinedConferenceGroup {
		// In-call conferenceGroup controls.
		if digit == "9" {
			gType := strings.ToLower(strings.TrimSpace(sess.conferenceGroupType))
			if gType == "mrd" {
				go s.ivrStartConferenceGroupFanout(key, sess)
			}
		}
		return
	}

	if digit == "#" {
		val := sess.buf.String()
		sess.buf.Reset()
		val = strings.TrimSpace(val)
		log.Printf("IVR commit key=%s state=%s value=%q", key, sess.state, val)
		switch sess.state {
		case ivrStateCollectBridge:
			sess.bridgeAccess = val
			sess.state = ivrStateCollectUser
			log.Printf("IVR state advance key=%s next_state=%s bridge_access=%q", key, sess.state, sess.bridgeAccess)
		case ivrStateCollectUser:
			sess.participantID = val
			kind, targetID, okJoin := s.ivrAuthorizeAndResolveTarget(sess.bridgeAccess, sess.participantID)
			log.Printf("IVR authorize key=%s bridge_access=%q participant_id=%q ok=%v kind=%s target_id=%q", key, sess.bridgeAccess, sess.participantID, okJoin, kind, targetID)
			if okJoin {
				cfgIVR := s.router.CurrentConfig()
				sess.preferredRegion = userRegionForParticipant(cfgIVR, val)
				var dialUserID, dialDisplay string
				var pinLen int
				pin := strings.TrimSpace(sess.participantID)
				for _, u := range cfgIVR.Spec.Users {
					if strings.TrimSpace(u.ParticipantID) == pin {
						dialUserID = u.ID
						dialDisplay = u.DisplayName
						pinLen = len(pin)
						break
					}
				}
				if kind == InviteTargetKindBridge {
					m := s.bridgeCalls[targetID]
					if m == nil {
						m = make(map[string]*bridgeCall)
						s.bridgeCalls[targetID] = m
					}
					m[key] = &bridgeCall{
						bridgeID:        targetID,
						callID:          callID,
						fromTag:         fromTag,
						toTag:           extractTag(sess.toHeader),
						fromHeader:      sess.fromHeader,
						toHeader:        sess.toHeader,
						contactURI:      sess.contactURI,
						remote:          sess.remote,
						createdAt:       sess.createdAt,
						userID:          dialUserID,
						userDisplayName: dialDisplay,
						pinLen:          pinLen,
					}
					sess.state = ivrStateJoined
					if sess.rtp != nil {
						sess.rtp.StartSilence()
					}
					delete(s.ivrSessions, key)
					log.Printf("IVR joined bridge key=%s bridge_id=%s", key, targetID)
					s.tryStartSIPRECForBridge(targetID, key)
					return
				}
				if kind == InviteTargetKindConferenceGroup {
					cfg := s.router.CurrentConfig()
					g, ok := conferenceGroupByID(cfg, targetID)
					if !ok {
						log.Printf("IVR join conferenceGroup failed: group not found group_id=%s", targetID)
						return
					}
					gType := strings.ToLower(strings.TrimSpace(g.Type))
					if gType == "" {
						gType = "mrd"
					}
					// Determine which side the caller is on (default Side A).
					callerSide := "A"
					fromUser := ExtractUserFromURI(ExtractURIFromAddressHeader(sess.fromHeader))
					if strings.TrimSpace(fromUser) != "" {
						for _, ep := range g.SideA {
							if ExtractUserFromURI(ep.SIPURI) == fromUser {
								callerSide = "A"
								break
							}
						}
						for _, ep := range g.SideB {
							if ExtractUserFromURI(ep.SIPURI) == fromUser {
								callerSide = "B"
								break
							}
						}
					}
					sess.conferenceGroupID = targetID
					sess.conferenceGroupType = gType
					sess.conferenceCallerSide = callerSide
					sess.state = ivrStateJoinedConferenceGroup
					if sess.rtp != nil {
						sess.rtp.StartSilence()
					}
					log.Printf("IVR joined conferenceGroup key=%s group_id=%s type=%s caller_side=%s", key, targetID, gType, callerSide)
					if gType == "ard" {
						go s.ivrStartConferenceGroupFanout(key, sess)
					}
					return
				}
			}
		}
		return
	}

	// numeric-only digits expected for access code / participant id
	if len(digit) == 1 {
		c := digit[0]
		if c >= '0' && c <= '9' {
			sess.buf.WriteByte(c)
			log.Printf("IVR buffer key=%s state=%s buf=%q", key, sess.state, sess.buf.String())
		}
	}
}

func (s *Server) ivrAuthorizeAndResolveTarget(accessCode, participantID string) (kind InviteTargetKind, targetID string, ok bool) {
	accessCode = strings.TrimSpace(accessCode)
	participantID = strings.TrimSpace(participantID)
	if accessCode == "" || participantID == "" {
		return "", "", false
	}
	if s.router == nil {
		return "", "", false
	}
	cfg := s.router.CurrentConfig()

	// Resolve access code to either a bridge or a conferenceGroup.
	bridgeID := ""
	for _, b := range cfg.Spec.Bridges {
		if !b.DDIAccessEnabled {
			continue
		}
		if strings.TrimSpace(b.DDIAccessNumber) == accessCode {
			bridgeID = b.ID
			break
		}
	}
	groupID := ""
	if bridgeID == "" {
		for _, g := range cfg.Spec.ConferenceGroups {
			if !g.DDIAccessEnabled {
				continue
			}
			if strings.TrimSpace(g.DDIAccessNumber) == accessCode {
				groupID = g.ID
				break
			}
		}
	}
	if bridgeID == "" && groupID == "" {
		return "", "", false
	}

	// Find user and check allow-lists.
	for _, u := range cfg.Spec.Users {
		if strings.TrimSpace(u.ParticipantID) != participantID {
			continue
		}
		if bridgeID != "" {
			for _, allowed := range u.AllowedBridgeIDs {
				if allowed == bridgeID {
					return InviteTargetKindBridge, bridgeID, true
				}
			}
			return "", "", false
		}
		for _, allowed := range u.AllowedConferenceGroupIDs {
			if allowed == groupID {
				return InviteTargetKindConferenceGroup, groupID, true
			}
		}
		return "", "", false
	}
	return "", "", false
}

func (s *Server) handleResponse(msg *Message) {
	callID := strings.TrimSpace(msg.Header("call-id"))
	if callID == "" {
		return
	}
	status := msg.StatusCode
	if status == 0 {
		return
	}

	s.sessionMu.Lock()
	leg, ok := s.legsByCallID[callID]
	if !ok {
		s.sessionMu.Unlock()
		return
	}
	// SIPREC outbound INVITE toward recorder (metadata + optional SDP).
	if strings.HasPrefix(leg.sessionKey, "siprec|") {
		respStatus := msg.StatusCode
		if respStatus >= 200 && respStatus < 300 {
			leg.toHeader = msg.Header("to")
			ackTarget := leg.targetURI
			if c := strings.TrimSpace(msg.Header("contact")); c != "" {
				if u := strings.TrimSpace(ExtractURIFromAddressHeader(c)); u != "" {
					ackTarget = u
					leg.siprecByeURI = u
				}
			}
			localIP, localPort := s.localViaForLeg(leg)
			ack := BuildOutboundAck(ackTarget, localIP, localPort, leg.callID, leg.fromTag, extractTag(leg.toHeader), leg.viaTransport)
			_ = s.writeLegOutbound(leg, ack)
			log.Printf("SIPREC recording session established call_id=%s", leg.callID)
		} else if respStatus >= 400 {
			logicalKey := strings.TrimPrefix(leg.sessionKey, "siprec|")
			delete(s.siprecRecordings, logicalKey)
			delete(s.legsByCallID, callID)
			log.Printf("SIPREC INVITE failed status=%d call_id=%s", respStatus, leg.callID)
		}
		s.sessionMu.Unlock()
		return
	}
	// ConferenceGroup dial-outs initiated from IVR use a separate session map.
	if strings.HasPrefix(leg.sessionKey, "ivrconf|") {
		fs, ok := s.ivrConfSessions[leg.sessionKey]
		if !ok || fs.terminated {
			s.sessionMu.Unlock()
			return
		}
		status := msg.StatusCode
		if status == 0 {
			s.sessionMu.Unlock()
			return
		}
		if status >= 200 && status < 300 {
			if fs.winnerCallID == "" {
				fs.winnerCallID = callID
				leg.toHeader = msg.Header("to")
				winnerLeg := leg
				legs := append([]*fanoutLeg(nil), fs.legs...)
				// Capture the OK SDP for RTP negotiation.
				okSDP := string(msg.Body)
				s.sessionMu.Unlock()

				// ACK winner and CANCEL others.
				localIP, localPort := s.localViaForLeg(winnerLeg)
				// ACK should be sent to the remote target (Contact) for this dialog.
				ackTarget := winnerLeg.targetURI
				if c := strings.TrimSpace(msg.Header("contact")); c != "" {
					if u := strings.TrimSpace(ExtractURIFromAddressHeader(c)); u != "" {
						ackTarget = u
					}
				}
				ack := BuildOutboundAck(ackTarget, localIP, localPort, winnerLeg.callID, winnerLeg.fromTag, extractTag(winnerLeg.toHeader), winnerLeg.viaTransport)
				_ = s.writeLegOutbound(winnerLeg, ack)
				for _, l := range legs {
					if l.callID == callID {
						continue
					}
					li, lp := s.localViaForLeg(l)
					cancel := BuildOutboundCancel(l.targetURI, li, lp, l.callID, l.branch, l.fromTag, l.viaTransport)
					_ = s.writeLegOutbound(l, cancel)
				}
				// Wire up RTP relay between IVR caller RTP and answered endpoint RTP.
				ivrKey := fs.ivrKey
				s.sessionMu.Lock()
				ivrSess := s.ivrSessions[ivrKey]
				s.sessionMu.Unlock()
				if ivrSess != nil && ivrSess.rtp != nil && winnerLeg.rtp != nil {
					offer, okOffer := parseSDPAudioOffer(okSDP, nil)
					if okOffer {
						winnerLeg.rtp.mu.Lock()
						winnerLeg.rtp.remote = &net.UDPAddr{IP: offer.Addr, Port: offer.Port}
						winnerLeg.rtp.mu.Unlock()
						winnerLeg.rtp.StartReceiver()
						ivrSess.rtp.StartReceiver()
						winnerLeg.rtp.SetOnRTPPacket(func(pkt []byte) {
							_, _ = ivrSess.rtp.conn.WriteToUDP(pkt, ivrSess.rtp.remote)
						})
						ivrSess.rtp.SetOnRTPPacket(func(pkt []byte) {
							_, _ = winnerLeg.rtp.conn.WriteToUDP(pkt, winnerLeg.rtp.remote)
						})
						log.Printf("IVR conferenceGroup RTP relay established ivr_key=%s winner_call_id=%s rtp_remote=%s", ivrKey, winnerLeg.callID, winnerLeg.rtp.remote.String())
						s.tryStartSIPRECForIVRConference(ivrKey, ivrSess)
					} else {
						log.Printf("IVR conferenceGroup RTP relay skipped: cannot parse OK SDP winner_call_id=%s", winnerLeg.callID)
					}
				} else {
					log.Printf("IVR conferenceGroup RTP relay not ready ivr_rtp=%v winner_rtp=%v", ivrSess != nil && ivrSess.rtp != nil, winnerLeg.rtp != nil)
				}

				log.Printf("IVR conferenceGroup fanout winner call_id=%s target=%s", callID, winnerLeg.targetURI)
				return
			}
		}
		s.sessionMu.Unlock()
		return
	}

	sess, ok := s.sessions[leg.sessionKey]
	if !ok || sess.terminated {
		s.sessionMu.Unlock()
		return
	}

	if status >= 200 {
		leg.finalStatus = status
	}

	// First 2xx wins.
	if status >= 200 && status < 300 {
		if sess.winnerCallID == "" {
			sess.winnerCallID = callID
		}
	}

	// Determine if all legs have final responses.
	allFinal := true
	for _, l := range sess.legs {
		if l.finalStatus == 0 {
			allFinal = false
		}
	}

	winner := sess.winnerCallID
	bridgeID := sess.bridgeID
	remote := sess.inboundRemote
	inbound := sess.inboundInvite
	ringTimeout := sess.ringTimeout

	// Mark terminated if winner chosen (we'll answer inbound and cancel losers).
	if winner != "" {
		sess.terminated = true
	}

	// Also terminate if everything failed.

	if winner != "" {
		localPort := s.localPortForInbound(sess)
		advertiseIPStr := s.advertisedIPForInbound(sess)
		toWithTag, toTag := ensureToTagWithValue(inbound.Header("to"))
		sdp := buildMinimalSDP(advertiseIPStr, localPort)
		contactTx := "udp"
		if sess.inboundBC.Conn != nil {
			contactTx = "tls"
		}
		extra := map[string]string{
			"Contact":      fmt.Sprintf("<sip:sipbridge@%s;transport=%s>", net.JoinHostPort(advertiseIPStr, strconv.Itoa(localPort)), contactTx),
			"Content-Type": "application/sdp",
			"Allow":        "INVITE, ACK, BYE, CANCEL, OPTIONS, INFO, REFER",
			"Supported":    "replaces, norefersub, timer",
			"To":           toWithTag,
		}
		okResp, _ := BuildResponse(inbound, 200, "OK", extra, []byte(sdp))
		_ = sess.inboundBC.Write(okResp)

		if strings.TrimSpace(bridgeID) != "" {
			fromHdr := inbound.Header("from")
			contactURI := ExtractURIFromAddressHeader(inbound.Header("contact"))
			fromTag := extractTag(fromHdr)
			sessKey := strings.TrimSpace(inbound.Header("call-id")) + "|" + fromTag
			s.sessionMu.Lock()
			m := s.bridgeCalls[bridgeID]
			if m == nil {
				m = make(map[string]*bridgeCall)
				s.bridgeCalls[bridgeID] = m
			}
			m[sessKey] = &bridgeCall{
				bridgeID:    bridgeID,
				callID:      strings.TrimSpace(inbound.Header("call-id")),
				fromTag:     fromTag,
				toTag:       toTag,
				fromHeader:  fromHdr,
				toHeader:    toWithTag,
				contactURI:  contactURI,
				remote:      remote,
				createdAt:   time.Now().UTC(),
			}
			s.sessionMu.Unlock()
			s.tryStartSIPRECForBridge(bridgeID, sessKey)
		} else if strings.TrimSpace(sess.ConferenceGroupID) != "" && sess.ConferenceARD {
			gid := strings.TrimSpace(sess.ConferenceGroupID)
			ardBridgeID := syntheticARDBridgeID(gid)
			fromHdr := inbound.Header("from")
			contactURI := ExtractURIFromAddressHeader(inbound.Header("contact"))
			fromTag := extractTag(fromHdr)
			sessKey := strings.TrimSpace(inbound.Header("call-id")) + "|" + fromTag
			s.sessionMu.Lock()
			m := s.bridgeCalls[ardBridgeID]
			if m == nil {
				m = make(map[string]*bridgeCall)
				s.bridgeCalls[ardBridgeID] = m
			}
			m[sessKey] = &bridgeCall{
				bridgeID:   ardBridgeID,
				callID:     strings.TrimSpace(inbound.Header("call-id")),
				fromTag:    fromTag,
				toTag:      toTag,
				fromHeader: fromHdr,
				toHeader:   toWithTag,
				contactURI: contactURI,
				remote:     remote,
				createdAt:  time.Now().UTC(),
			}
			s.sessionMu.Unlock()
			s.tryStartSIPRECForBridge(ardBridgeID, sessKey)
		}

		s.cancelLosers(inbound, winner)
		s.cleanupSession(inbound)
		_ = ringTimeout
		return
	}

	if allFinal {
		fail, _ := BuildResponse(inbound, 480, "Temporarily Unavailable", nil, nil)
		_ = sess.inboundBC.Write(fail)
		s.cleanupSession(inbound)
	}
}

func (s *Server) enforceRingTimeout(sessKey string, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	<-t.C

	s.sessionMu.Lock()
	sess, ok := s.sessions[sessKey]
	if !ok || sess.terminated {
		s.sessionMu.Unlock()
		return
	}
	sess.terminated = true
	inbound := sess.inboundInvite
	legs := append([]*fanoutLeg(nil), sess.legs...)
	inboundBC := sess.inboundBC
	s.sessionMu.Unlock()

	for _, l := range legs {
		li, lp := s.localViaForLeg(l)
		cancel := BuildOutboundCancel(l.targetURI, li, lp, l.callID, l.branch, l.fromTag, l.viaTransport)
		_ = s.writeLegOutbound(l, cancel)
	}

	timeout, _ := BuildResponse(inbound, 408, "Request Timeout", nil, nil)
	_ = inboundBC.Write(timeout)

	s.cleanupSession(inbound)
}

func (s *Server) handleInboundCancel(msg *Message, conn *net.UDPConn, remote *net.UDPAddr) {
	bc := sipBackchannel{UDP: conn, Peer: remote}
	okResp, _ := BuildResponse(msg, 200, "OK", nil, nil)
	_ = bc.Write(okResp)

	callID := strings.TrimSpace(msg.Header("call-id"))
	fromTag := extractTag(msg.Header("from"))
	if callID == "" || fromTag == "" {
		return
	}
	bridgeKey := callID + "|" + fromTag
	key := callID + "|" + fromTag

	s.sessionMu.Lock()
	ivrSess := s.ivrSessions[key]
	if ivrSess != nil {
		delete(s.ivrSessions, key)
	}
	s.sessionMu.Unlock()
	if ivrSess != nil && ivrSess.rtp != nil {
		ivrSess.rtp.Close()
	}
	s.stopSIPRECRecording("ivr:" + key)

	s.sessionMu.Lock()
	sess, ok := s.sessions[key]
	if !ok || sess.terminated {
		s.sessionMu.Unlock()
		s.cleanupBridgeCallByKey(bridgeKey)
		return
	}
	sess.terminated = true
	inboundInvite := sess.inboundInvite
	legs := append([]*fanoutLeg(nil), sess.legs...)
	s.sessionMu.Unlock()

	for _, l := range legs {
		li, lp := s.localViaForLeg(l)
		cancel := BuildOutboundCancel(l.targetURI, li, lp, l.callID, l.branch, l.fromTag, l.viaTransport)
		_ = s.writeLegOutbound(l, cancel)
	}

	term, _ := BuildResponse(inboundInvite, 487, "Request Terminated", nil, nil)
	_ = bc.Write(term)

	s.cleanupSession(inboundInvite)
	s.cleanupBridgeCallByKey(bridgeKey)
}

func (s *Server) handleInboundBye(msg *Message, conn *net.UDPConn, remote *net.UDPAddr) {
	bc := sipBackchannel{UDP: conn, Peer: remote}
	okResp, _ := BuildResponse(msg, 200, "OK", nil, nil)
	_ = bc.Write(okResp)

	callID := strings.TrimSpace(msg.Header("call-id"))
	fromTag := extractTag(msg.Header("from"))
	if callID == "" || fromTag == "" {
		return
	}
	bridgeKey := callID + "|" + fromTag
	key := callID + "|" + fromTag

	s.sessionMu.Lock()
	ivrSess := s.ivrSessions[key]
	if ivrSess != nil {
		delete(s.ivrSessions, key)
	}
	s.sessionMu.Unlock()
	if ivrSess != nil && ivrSess.rtp != nil {
		ivrSess.rtp.Close()
	}
	s.stopSIPRECRecording("ivr:" + key)

	s.sessionMu.Lock()
	sess, ok := s.sessions[key]
	if !ok {
		s.sessionMu.Unlock()
		s.cleanupBridgeCallByKey(bridgeKey)
		return
	}
	sess.terminated = true
	winnerCallID := sess.winnerCallID
	legs := append([]*fanoutLeg(nil), sess.legs...)
	s.sessionMu.Unlock()

	if winnerCallID != "" {
		for _, l := range legs {
			if l.callID != winnerCallID {
				continue
			}
			li, lp := s.localViaForLeg(l)
			bye := BuildOutboundBye(l.targetURI, li, lp, l.callID, l.branch, l.fromTag, l.viaTransport)
			_ = s.writeLegOutbound(l, bye)
			break
		}
	} else {
		for _, l := range legs {
			li, lp := s.localViaForLeg(l)
			cancel := BuildOutboundCancel(l.targetURI, li, lp, l.callID, l.branch, l.fromTag, l.viaTransport)
			_ = s.writeLegOutbound(l, cancel)
		}
	}

	s.cleanupSession(msg)
	s.cleanupBridgeCallByKey(bridgeKey)
}

func (s *Server) cleanupBridgeCallByKey(key string) {
	s.sessionMu.Lock()
	var bridgeID string
	var found bool
	for bid, m := range s.bridgeCalls {
		if c, ok := m[key]; ok {
			found = true
			bridgeID = bid
			if c != nil && c.rtp != nil {
				c.rtp.Close()
				c.rtp = nil
			}
			delete(m, key)
			if len(m) == 0 {
				delete(s.bridgeCalls, bid)
			}
			break
		}
	}
	s.sessionMu.Unlock()
	if found {
		s.stopSIPRECRecording("bridge:" + bridgeID + ":" + key)
	}
}

// tryHandleBridgeReinvite handles in-dialog INVITE (RE-INVITE) for an established bridge leg.
func (s *Server) tryHandleBridgeReinvite(msg *Message, conn *net.UDPConn, remote *net.UDPAddr, advertiseIP string, localPort int, sessKey, toHdr string) bool {
	s.sessionMu.Lock()
	var bc *bridgeCall
	var bridgeID string
	for bid, m := range s.bridgeCalls {
		if c, ok := m[sessKey]; ok {
			bc, bridgeID = c, bid
			break
		}
	}
	if bc == nil {
		s.sessionMu.Unlock()
		return false
	}
	reqToTag := extractTag(toHdr)
	if reqToTag == "" || bc.toTag == "" || reqToTag != bc.toTag {
		s.sessionMu.Unlock()
		resp, _ := BuildResponse(msg, 481, "Call/Transaction Does Not Exist", nil, nil)
		_, _ = conn.WriteToUDP(resp, remote)
		return true
	}
	if contactURI := ExtractURIFromAddressHeader(msg.Header("contact")); contactURI != "" {
		bc.contactURI = contactURI
	}
	if remote != nil {
		bc.remote = remote
	}
	body := strings.TrimSpace(string(msg.Body))
	if body == "" {
		s.sessionMu.Unlock()
		toWithTag, _ := ensureToTagWithValue(msg.Header("to"))
		extra := map[string]string{
			"Contact":   fmt.Sprintf("<sip:sipbridge@%s;transport=udp>", net.JoinHostPort(advertiseIP, strconv.Itoa(localPort))),
			"Allow":     "INVITE, ACK, BYE, CANCEL, OPTIONS, UPDATE, INFO, REFER",
			"Supported": "timer, replaces, norefersub",
			"To":        toWithTag,
		}
		resp, _ := BuildResponse(msg, 200, "OK", extra, nil)
		_, _ = conn.WriteToUDP(resp, remote)
		log.Printf("bridge RE-INVITE 200 (no SDP) bridge_id=%s sess_key=%s", bridgeID, sessKey)
		return true
	}
	offer, okOffer := parseSDPAudioOffer(body, remote.IP)
	var sdp string
	if okOffer && offer.HasPCMU {
		if bc.rtp == nil {
			pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
			if err != nil {
				s.sessionMu.Unlock()
				resp, _ := BuildResponse(msg, 488, "Not Acceptable Here", nil, nil)
				_, _ = conn.WriteToUDP(resp, remote)
				return true
			}
			telPT := uint8(0)
			if offer.TelephoneEventPT > 0 && offer.TelephoneEventPT < 128 {
				telPT = uint8(offer.TelephoneEventPT)
			}
			remoteRTPIP := offer.Addr
			if remote != nil && remote.IP != nil && !remote.IP.IsUnspecified() {
				remoteRTPIP = remote.IP
			}
			rtpSess := newRTPSession(pc, &net.UDPAddr{IP: remoteRTPIP, Port: offer.Port}, 0, telPT, nil)
			rtpSess.StartReceiver()
			bc.rtp = rtpSess
		} else {
			remoteRTPIP := offer.Addr
			if remote != nil && remote.IP != nil && !remote.IP.IsUnspecified() {
				remoteRTPIP = remote.IP
			}
			bc.rtp.mu.Lock()
			bc.rtp.remote = &net.UDPAddr{IP: remoteRTPIP, Port: offer.Port}
			bc.rtp.mu.Unlock()
		}
		rtpPort := bc.rtp.LocalPort()
		sdp = buildIVRSDPAnswer(advertiseIP, rtpPort, offer.TelephoneEventPT)
	} else {
		if bc.rtp != nil {
			bc.rtp.Close()
			bc.rtp = nil
		}
		sdp = buildMinimalSDP(advertiseIP, localPort)
	}
	toWithTag, _ := ensureToTagWithValue(msg.Header("to"))
	extra := map[string]string{
		"Contact":      fmt.Sprintf("<sip:sipbridge@%s;transport=udp>", net.JoinHostPort(advertiseIP, strconv.Itoa(localPort))),
		"Content-Type": "application/sdp",
		"Allow":        "INVITE, ACK, BYE, CANCEL, OPTIONS, UPDATE, INFO, REFER",
		"Supported":    "timer, replaces, norefersub",
		"To":           toWithTag,
	}
	s.sessionMu.Unlock()
	resp, _ := BuildResponse(msg, 200, "OK", extra, []byte(sdp))
	_, _ = conn.WriteToUDP(resp, remote)
	log.Printf("bridge RE-INVITE 200 bridge_id=%s sess_key=%s", bridgeID, sessKey)
	return true
}

// tryHandleIVRReinvite handles in-dialog INVITE for an established IVR session.
func (s *Server) tryHandleIVRReinvite(msg *Message, conn *net.UDPConn, remote *net.UDPAddr, advertiseIP string, localPort int, sessKey, toHdr string) bool {
	s.sessionMu.Lock()
	sess := s.ivrSessions[sessKey]
	if sess == nil {
		s.sessionMu.Unlock()
		return false
	}
	reqToTag := extractTag(toHdr)
	sessToTag := extractTag(sess.toHeader)
	if reqToTag == "" || sessToTag == "" || reqToTag != sessToTag {
		s.sessionMu.Unlock()
		resp, _ := BuildResponse(msg, 481, "Call/Transaction Does Not Exist", nil, nil)
		_, _ = conn.WriteToUDP(resp, remote)
		return true
	}
	if contactURI := ExtractURIFromAddressHeader(msg.Header("contact")); contactURI != "" {
		sess.contactURI = contactURI
	}
	sess.remote = remote
	body := strings.TrimSpace(string(msg.Body))
	if body == "" {
		s.sessionMu.Unlock()
		toWithTag, _ := ensureToTagWithValue(msg.Header("to"))
		extra := map[string]string{
			"Contact":   fmt.Sprintf("<sip:sipbridge@%s;transport=udp>", net.JoinHostPort(advertiseIP, strconv.Itoa(localPort))),
			"Allow":     "INVITE, ACK, BYE, CANCEL, OPTIONS, UPDATE, INFO, REFER",
			"Supported": "replaces, norefersub",
			"To":        toWithTag,
		}
		resp, _ := BuildResponse(msg, 200, "OK", extra, nil)
		_, _ = conn.WriteToUDP(resp, remote)
		log.Printf("IVR RE-INVITE 200 (no SDP) sess_key=%s", sessKey)
		return true
	}
	offer, okOffer := parseSDPAudioOffer(body, remote.IP)
	var sdp string
	if okOffer && offer.HasPCMU {
		if sess.rtp == nil {
			pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
			if err != nil {
				s.sessionMu.Unlock()
				resp, _ := BuildResponse(msg, 488, "Not Acceptable Here", nil, nil)
				_, _ = conn.WriteToUDP(resp, remote)
				return true
			}
			telPT := uint8(0)
			if offer.TelephoneEventPT > 0 && offer.TelephoneEventPT < 128 {
				telPT = uint8(offer.TelephoneEventPT)
			}
			remoteRTPIP := offer.Addr
			if remote != nil && remote.IP != nil && !remote.IP.IsUnspecified() {
				remoteRTPIP = remote.IP
			}
			rtpSess := newRTPSession(pc, &net.UDPAddr{IP: remoteRTPIP, Port: offer.Port}, 0, telPT, func(d string) {
				parts := strings.SplitN(sessKey, "|", 2)
				callID := ""
				fromTag := ""
				if len(parts) == 2 {
					callID = parts[0]
					fromTag = parts[1]
				}
				s.sessionMu.Lock()
				s2 := s.ivrSessions[sessKey]
				if s2 != nil {
					s.handleIVRDigitLocked(sessKey, s2, d, callID, fromTag)
				}
				s.sessionMu.Unlock()
			})
			rtpSess.StartReceiver()
			sess.rtp = rtpSess
			sess.dtmfViaRTP = offer.TelephoneEventPT > 0
		} else {
			remoteRTPIP := offer.Addr
			if remote != nil && remote.IP != nil && !remote.IP.IsUnspecified() {
				remoteRTPIP = remote.IP
			}
			sess.rtp.mu.Lock()
			sess.rtp.remote = &net.UDPAddr{IP: remoteRTPIP, Port: offer.Port}
			sess.rtp.mu.Unlock()
		}
		rtpPort := localPort
		if sess.rtp != nil {
			rtpPort = sess.rtp.LocalPort()
		}
		sdp = buildIVRSDPAnswer(advertiseIP, rtpPort, offer.TelephoneEventPT)
	} else {
		rtpPort := localPort
		if sess.rtp != nil {
			rtpPort = sess.rtp.LocalPort()
		}
		sdp = buildMinimalSDP(advertiseIP, rtpPort)
	}
	toWithTag, _ := ensureToTagWithValue(msg.Header("to"))
	extra := map[string]string{
		"Contact":      fmt.Sprintf("<sip:sipbridge@%s;transport=udp>", net.JoinHostPort(advertiseIP, strconv.Itoa(localPort))),
		"Content-Type": "application/sdp",
		"Allow":        "INVITE, ACK, BYE, CANCEL, OPTIONS, UPDATE, INFO, REFER",
		"Supported":    "replaces, norefersub",
		"To":           toWithTag,
	}
	s.sessionMu.Unlock()
	resp, _ := BuildResponse(msg, 200, "OK", extra, []byte(sdp))
	_, _ = conn.WriteToUDP(resp, remote)
	log.Printf("IVR RE-INVITE 200 sess_key=%s", sessKey)
	return true
}

func sessRemoteFromIVR(m map[string]*ivrSession, ivrKey string) *net.UDPAddr {
	if m == nil {
		return nil
	}
	if s := m[ivrKey]; s != nil {
		return s.remote
	}
	return nil
}

func (s *Server) cancelLosers(inbound *Message, winnerCallID string) {
	callID := strings.TrimSpace(inbound.Header("call-id"))
	fromTag := extractTag(inbound.Header("from"))
	if callID == "" || fromTag == "" {
		return
	}
	key := callID + "|" + fromTag

	s.sessionMu.Lock()
	sess, ok := s.sessions[key]
	if !ok {
		s.sessionMu.Unlock()
		return
	}
	legs := append([]*fanoutLeg(nil), sess.legs...)
	s.sessionMu.Unlock()

	for _, l := range legs {
		if l.callID == winnerCallID {
			continue
		}
		li, lp := s.localViaForLeg(l)
		cancel := BuildOutboundCancel(l.targetURI, li, lp, l.callID, l.branch, l.fromTag, l.viaTransport)
		_ = s.writeLegOutbound(l, cancel)
	}
}

func (s *Server) outboundDest(fallback *net.UDPAddr) *net.UDPAddr {
	addr := strings.TrimSpace(s.cfg.OutboundProxyAddr)
	port := s.cfg.OutboundProxyPort
	if addr == "" || port <= 0 {
		return fallback
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return fallback
	}
	return &net.UDPAddr{IP: ip, Port: port}
}

func (s *Server) outboundDestForTarget(targetURI string, fallback *net.UDPAddr) *net.UDPAddr {
	// If an explicit outbound proxy is configured, always use it.
	if dst := s.outboundDest(nil); dst != nil {
		return dst
	}

	// Parse sip:[user@]host[:port][;params]
	// Note: net/url parses "sip:alice@example.com" into Scheme=sip, Opaque="alice@example.com".
	raw := strings.TrimSpace(targetURI)
	raw = strings.TrimPrefix(raw, "<")
	raw = strings.TrimSuffix(raw, ">")

	if i := strings.IndexByte(raw, ';'); i >= 0 {
		raw = raw[:i]
	}

	var hostport string
	if u, err := url.Parse(raw); err == nil {
		if u.Host != "" {
			hostport = u.Host
		} else if u.Opaque != "" {
			hostport = u.Opaque
		} else {
			hostport = raw
		}
	} else {
		hostport = raw
	}

	if at := strings.LastIndex(hostport, "@"); at >= 0 {
		hostport = hostport[at+1:]
	}

	hostport = strings.TrimPrefix(hostport, "//")
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return fallback
	}

	host := hostport
	port := 5060
	if h, p, err := net.SplitHostPort(hostport); err == nil {
		host = h
		if pp, convErr := strconv.Atoi(p); convErr == nil {
			port = pp
		}
	} else {
		// If no explicit port is provided, default to 5060.
		// Handle IPv6 literals without port is ambiguous; fall back.
		if strings.HasPrefix(hostport, "[") {
			return fallback
		}
	}

	if ip := net.ParseIP(host); ip != nil {
		return &net.UDPAddr{IP: ip, Port: port}
	}

	// Avoid long DNS stalls for unroutable or reserved domains.
	if strings.HasSuffix(strings.ToLower(host), ".invalid") {
		return fallback
	}

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return fallback
	}
	for _, ipa := range ips {
		if ipa.IP == nil {
			continue
		}
		if v4 := ipa.IP.To4(); v4 != nil {
			return &net.UDPAddr{IP: v4, Port: port}
		}
	}
	// Fallback to first IP (IPv6) if no IPv4.
	if ips[0].IP != nil {
		return &net.UDPAddr{IP: ips[0].IP, Port: port}
	}
	return fallback
}

func (s *Server) cleanupSession(inbound *Message) {
	callID := strings.TrimSpace(inbound.Header("call-id"))
	fromTag := extractTag(inbound.Header("from"))
	if callID == "" || fromTag == "" {
		return
	}
	key := callID + "|" + fromTag

	s.sessionMu.Lock()
	sess, ok := s.sessions[key]
	if ok {
		for _, l := range sess.legs {
			delete(s.legsByCallID, l.callID)
			if l.outboundConn != nil {
				_ = l.outboundConn.Close()
				l.outboundConn = nil
			}
		}
		delete(s.sessions, key)
	}
	s.sessionMu.Unlock()
}

func extractTag(h string) string {
	low := strings.ToLower(h)
	idx := strings.Index(low, "tag=")
	if idx < 0 {
		return ""
	}
	t := h[idx+4:]
	if cut := strings.IndexAny(t, ";> "); cut >= 0 {
		t = t[:cut]
	}
	return strings.TrimSpace(t)
}

func ensureToTagWithValue(to string) (withTag string, tag string) {
	withTag = ensureToTag(to)
	tag = extractTag(withTag)
	return withTag, tag
}

func buildInboundDialogBye(targetURI, localIP string, localPort int, callID, fromHeader, toHeader string) []byte {
	branch := "z9hG4bK" + RandHex(12)
	localHostPort := net.JoinHostPort(localIP, strconv.Itoa(localPort))

	b := &strings.Builder{}
	fmt.Fprintf(b, "BYE %s SIP/2.0\r\n", targetURI)
	fmt.Fprintf(b, "Via: SIP/2.0/UDP %s;branch=%s\r\n", localHostPort, branch)
	b.WriteString("Max-Forwards: 70\r\n")
	// For an in-dialog request, From is us (callee) and To is the remote (caller).
	fmt.Fprintf(b, "From: %s\r\n", strings.TrimSpace(toHeader))
	fmt.Fprintf(b, "To: %s\r\n", strings.TrimSpace(fromHeader))
	fmt.Fprintf(b, "Call-ID: %s\r\n", callID)
	b.WriteString("CSeq: 2 BYE\r\n")
	b.WriteString("Content-Length: 0\r\n\r\n")
	return []byte(b.String())
}

func advertisedIP(conn *net.UDPConn, remote *net.UDPAddr) string {
	// Prefer the bound local address if it's a concrete, non-unspecified IP.
	if conn != nil {
		if la, ok := conn.LocalAddr().(*net.UDPAddr); ok && la != nil {
			ip := la.IP
			if ip != nil && !ip.IsUnspecified() {
				return ip.String()
			}
		}
	}

	// If we are bound to 0.0.0.0 / ::, advertise the interface/address that the caller used.
	if remote != nil && remote.IP != nil && !remote.IP.IsUnspecified() && !remote.IP.IsLoopback() {
		return remote.IP.String()
	}

	// If the remote is loopback/unspecified, pick a non-loopback interface address.
	// Prefer RFC1918 IPv4 (10/8, 172.16/12, 192.168/16) and avoid APIPA link-local (169.254/16).
	isRFC1918 := func(ip net.IP) bool {
		v4 := ip.To4()
		if v4 == nil {
			return false
		}
		switch {
		case v4[0] == 10:
			return true
		case v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31:
			return true
		case v4[0] == 192 && v4[1] == 168:
			return true
		default:
			return false
		}
	}
	isLinkLocal := func(ip net.IP) bool {
		v4 := ip.To4()
		return v4 != nil && v4[0] == 169 && v4[1] == 254
	}

	var fallbackIPv4 string
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet == nil || ipnet.IP == nil {
				continue
			}
			ip := ipnet.IP
			if ip.IsLoopback() || ip.IsUnspecified() {
				continue
			}
			v4 := ip.To4()
			if v4 == nil {
				continue
			}
			if isLinkLocal(v4) {
				continue
			}
			if isRFC1918(v4) {
				return v4.String()
			}
			if fallbackIPv4 == "" {
				fallbackIPv4 = v4.String()
			}
		}
	}
	if fallbackIPv4 != "" {
		return fallbackIPv4
	}

	return "127.0.0.1"
}

func buildMinimalSDP(ip string, port int) string {
	// Minimal SDP with inactive RTP. This is a signaling baseline only; real media anchoring/mixer comes next.
	family := "IP4"
	if parsed := net.ParseIP(strings.TrimSpace(ip)); parsed != nil {
		if parsed.To4() == nil {
			family = "IP6"
		}
	}

	var b strings.Builder
	b.WriteString("v=0\r\n")
	fmt.Fprintf(&b, "o=sipbridge 0 0 IN %s %s\r\n", family, ip)
	b.WriteString("s=SIPBridge\r\n")
	fmt.Fprintf(&b, "c=IN %s %s\r\n", family, ip)
	b.WriteString("t=0 0\r\n")
	b.WriteString("m=audio ")
	b.WriteString(strconv.Itoa(port))
	b.WriteString(" RTP/AVP 0 8\r\n")
	b.WriteString("a=rtpmap:0 PCMU/8000\r\n")
	b.WriteString("a=rtpmap:8 PCMA/8000\r\n")
	b.WriteString("a=inactive\r\n")
	return b.String()
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.udpConn != nil {
		_ = s.udpConn.Close()
		s.udpConn = nil
	}
	s.started = false
	return nil
}

func (s *Server) SIPConfig() config.SIPConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Server) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Stats{
		Started:      s.started,
		PacketsRx:    s.packetsRx,
		BytesRx:      s.bytesRx,
		LastPacketAt: s.lastPacketAt,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
