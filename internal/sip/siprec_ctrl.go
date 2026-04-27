package sip

import (
	"log"
	"net"
	"strings"
	"time"

	"sipbridge/internal/config"
	"sipbridge/internal/siprec"
)

func bridgeRecordingEnabled(b config.Bridge) bool {
	if b.RecordingEnabled == nil {
		return true
	}
	return *b.RecordingEnabled
}

func conferenceGroupRecordingEnabled(g config.ConferenceGroup) bool {
	if g.RecordingEnabled == nil {
		return true
	}
	return *g.RecordingEnabled
}

func pickRecordingInviteURI(t config.RecordingTrunkEntry) string {
	if u := strings.TrimSpace(t.RecordingTrunkSIPURI); u != "" {
		return u
	}
	return strings.TrimSpace(t.RecorderSIPURI)
}

func conferenceEndpointLineLabel(cfg config.RootConfig, g config.ConferenceGroup, fromHeader string) string {
	fromUser := strings.TrimSpace(ExtractUserFromURI(ExtractURIFromAddressHeader(fromHeader)))
	if fromUser == "" {
		return ""
	}
	match := func(ep config.Endpoint) bool {
		sp := strings.TrimSpace(ExtractUserFromURI(ep.SIPURI))
		return sp != "" && strings.EqualFold(sp, fromUser)
	}
	for _, ep := range g.SideA {
		if match(ep) {
			return strings.TrimSpace(ep.LineLabel)
		}
	}
	for _, ep := range g.SideB {
		if match(ep) {
			return strings.TrimSpace(ep.LineLabel)
		}
	}
	return ""
}

func conferenceEndpointDisplayName(cfg config.RootConfig, g config.ConferenceGroup, fromHeader string) string {
	fromUser := strings.TrimSpace(ExtractUserFromURI(ExtractURIFromAddressHeader(fromHeader)))
	if fromUser == "" {
		return ""
	}
	match := func(ep config.Endpoint) bool {
		sp := strings.TrimSpace(ExtractUserFromURI(ep.SIPURI))
		return sp != "" && strings.EqualFold(sp, fromUser)
	}
	for _, ep := range g.SideA {
		if match(ep) {
			return strings.TrimSpace(ep.DisplayName)
		}
	}
	for _, ep := range g.SideB {
		if match(ep) {
			return strings.TrimSpace(ep.DisplayName)
		}
	}
	return ""
}

// bridgeLineLabelForFrom returns bridge line_label or the participant leg label matching From (private wire metadata).
func bridgeLineLabelForFrom(cfg config.RootConfig, bridgeID string, fromHeader string) string {
	br, ok := bridgeByID(cfg, bridgeID)
	if !ok {
		return ""
	}
	if s := strings.TrimSpace(br.LineLabel); s != "" {
		return s
	}
	fromUser := strings.TrimSpace(ExtractUserFromURI(ExtractURIFromAddressHeader(fromHeader)))
	if fromUser == "" {
		return ""
	}
	for i := range br.Participants {
		p := &br.Participants[i]
		su := strings.TrimSpace(ExtractUserFromURI(p.SIPURI))
		if su != "" && strings.EqualFold(su, fromUser) {
			return strings.TrimSpace(p.LineLabel)
		}
	}
	return ""
}

func applyLineLabelFromUserDevices(meta *siprec.ParticipantRecordingMeta, user *config.User) {
	if meta == nil || strings.TrimSpace(meta.LineLabel) != "" {
		return
	}
	if user == nil {
		return
	}
	for _, d := range user.Devices {
		if !strings.EqualFold(strings.TrimSpace(d.Kind), "private_wire") {
			continue
		}
		if d.CTI != nil {
			if s := strings.TrimSpace(d.CTI["line_label"]); s != "" {
				meta.LineLabel = s
				return
			}
		}
		if s := strings.TrimSpace(d.Address); s != "" {
			meta.LineLabel = s
			return
		}
	}
}

// conferenceInviteParticipantLabels returns linked user id and display name when From matches
// linked_user_id; otherwise display name from the matching SideA/SideB endpoint (for SIPREC metadata).
func conferenceInviteParticipantLabels(cfg config.RootConfig, g config.ConferenceGroup, fromHeader string) (userID, displayName string) {
	uid, disp := conferenceInvitedUserIdentity(cfg, g, fromHeader)
	if strings.TrimSpace(disp) != "" {
		return uid, disp
	}
	return uid, conferenceEndpointDisplayName(cfg, g, fromHeader)
}

// conferenceInvitedUserIdentity returns the linked user id and display name when the SIP From
// matches a conference endpoint with linked_user_id (for SIPREC metadata and participant opt-in).
func conferenceInvitedUserIdentity(cfg config.RootConfig, g config.ConferenceGroup, fromHeader string) (userID, displayName string) {
	fromUser := strings.TrimSpace(ExtractUserFromURI(ExtractURIFromAddressHeader(fromHeader)))
	if fromUser == "" {
		return "", ""
	}
	match := func(ep config.Endpoint) bool {
		sp := strings.TrimSpace(ExtractUserFromURI(ep.SIPURI))
		return sp != "" && strings.EqualFold(sp, fromUser)
	}
	var linked string
	for _, ep := range g.SideA {
		if match(ep) {
			linked = strings.TrimSpace(ep.LinkedUserID)
			break
		}
	}
	if linked == "" {
		for _, ep := range g.SideB {
			if match(ep) {
				linked = strings.TrimSpace(ep.LinkedUserID)
				break
			}
		}
	}
	if linked == "" {
		return "", ""
	}
	for i := range cfg.Spec.Users {
		u := &cfg.Spec.Users[i]
		if u.ID == linked {
			return u.ID, u.DisplayName
		}
	}
	return "", ""
}

func (s *Server) bridgeCallParticipantContext(bridgeID, bridgeKey string) (user *config.User, dialIn bool) {
	if s == nil || s.router == nil {
		return nil, false
	}
	s.sessionMu.Lock()
	var uid string
	var pinDialIn bool
	if m := s.bridgeCalls[bridgeID]; m != nil {
		if bc := m[bridgeKey]; bc != nil {
			uid = strings.TrimSpace(bc.userID)
			pinDialIn = bc.pinLen > 0
		}
	}
	s.sessionMu.Unlock()
	if uid == "" {
		return nil, pinDialIn
	}
	cfg := s.router.CurrentConfig()
	for i := range cfg.Spec.Users {
		u := &cfg.Spec.Users[i]
		if u.ID == uid {
			return u, pinDialIn
		}
	}
	return nil, pinDialIn
}

func (s *Server) tryStartSIPRECForIVRConference(ivrKey string, sess *ivrSession) {
	if s == nil || sess == nil || s.router == nil {
		return
	}
	cfg := s.router.CurrentConfig()
	root := cfg
	rec := root.Spec.Recording
	if rec == nil || !rec.GlobalEnabled || rec.SIPREC == nil || !rec.SIPREC.Enabled {
		return
	}
	gid := strings.TrimSpace(sess.conferenceGroupID)
	if gid == "" {
		return
	}
	g, ok := conferenceGroupByID(root, gid)
	if !ok {
		return
	}
	pin := strings.TrimSpace(sess.participantID)
	var user *config.User
	for i := range root.Spec.Users {
		u := &root.Spec.Users[i]
		if strings.TrimSpace(u.ParticipantID) == pin {
			user = u
			break
		}
	}
	if !conferenceGroupRecordingEnabled(g) && (user == nil || !user.RecordingOptIn) {
		log.Printf("SIPREC skip ivr conference group_id=%s: conference recording off and participant recording opt-in false or unknown user", gid)
		return
	}
	siprecSpec := rec.SIPREC
	trunk, ok := config.SelectRecordingTrunkForRegion(siprecSpec, sess.preferredRegion)
	if !ok || strings.TrimSpace(trunk.RecorderSIPURI) == "" {
		return
	}

	meta := siprec.ParticipantRecordingMeta{
		ConferenceGroupID: gid,
		ConferenceName:    strings.TrimSpace(g.Name),
		ParticipantPIN:    pin,
		LineLabel:         strings.TrimSpace(g.LineLabel),
		DialIn:            pin != "",
	}
	if user != nil {
		meta.EmployeeID = user.ID
		meta.DisplayName = user.DisplayName
	}
	if strings.TrimSpace(meta.LineLabel) == "" {
		fromH := sess.fromHeader
		if xl := conferenceEndpointLineLabel(root, g, fromH); xl != "" {
			meta.LineLabel = xl
		}
	}
	applyLineLabelFromUserDevices(&meta, user)
	xml, err := siprec.BuildMetadataXML(meta)
	if err != nil {
		log.Printf("SIPREC metadata: %v", err)
		return
	}

	inviteURI := pickRecordingInviteURI(trunk)
	logicalKey := "ivr:" + ivrKey
	if err := s.emitSIPRECInvite(logicalKey, inviteURI, xml); err != nil {
		log.Printf("SIPREC start ivr conference: %v", err)
	}
}

func (s *Server) tryStartSIPRECForBridge(bridgeID, bridgeKey string) {
	if s == nil || s.router == nil {
		return
	}
	cfg := s.router.CurrentConfig()
	rec := cfg.Spec.Recording
	if rec == nil {
		log.Printf("SIPREC skip bridge_id=%s: spec.recording is nil (save Settings → Recording or add spec.recording to config.yaml and restart)", bridgeID)
		return
	}
	if !rec.GlobalEnabled {
		log.Printf("SIPREC skip bridge_id=%s: global_enabled=false", bridgeID)
		return
	}
	if rec.SIPREC == nil || !rec.SIPREC.Enabled {
		log.Printf("SIPREC skip bridge_id=%s: siprec missing or disabled", bridgeID)
		return
	}
	siprecSpec := rec.SIPREC
	trunk, ok := config.SelectRecordingTrunkForRegion(siprecSpec, "")
	if !ok || strings.TrimSpace(trunk.RecorderSIPURI) == "" {
		log.Printf("SIPREC skip bridge_id=%s: no recorder_sip_uri (check legacy URI or trunks/default_trunk_id)", bridgeID)
		return
	}
	user, dialIn := s.bridgeCallParticipantContext(bridgeID, bridgeKey)
	var bcDisp, bcPIN, bcLine, fromHdr string
	s.sessionMu.Lock()
	if m := s.bridgeCalls[bridgeID]; m != nil {
		if bc := m[bridgeKey]; bc != nil {
			bcDisp = strings.TrimSpace(bc.userDisplayName)
			bcPIN = strings.TrimSpace(bc.participantPIN)
			bcLine = strings.TrimSpace(bc.lineLabel)
			fromHdr = bc.fromHeader
		}
	}
	s.sessionMu.Unlock()
	meta := siprec.ParticipantRecordingMeta{DialIn: dialIn}
	if user != nil {
		meta.EmployeeID = user.ID
		meta.DisplayName = user.DisplayName
	} else if bcDisp != "" {
		meta.DisplayName = bcDisp
	}
	if bcPIN != "" {
		meta.ParticipantPIN = bcPIN
	}
	if bcLine != "" {
		meta.LineLabel = bcLine
	}
	if strings.HasPrefix(bridgeID, ardGroupBridgePrefix) {
		gid := strings.TrimPrefix(bridgeID, ardGroupBridgePrefix)
		g, ok := conferenceGroupByID(cfg, gid)
		if !ok {
			log.Printf("SIPREC skip bridge_id=%s: conference group not found", bridgeID)
			return
		}
		if !conferenceGroupRecordingEnabled(g) && (user == nil || !user.RecordingOptIn) {
			log.Printf("SIPREC skip bridge_id=%s: conference group recording off and participant recording opt-in false or unknown user", bridgeID)
			return
		}
		meta.ConferenceGroupID = gid
		meta.ConferenceName = strings.TrimSpace(g.Name)
		if strings.TrimSpace(meta.LineLabel) == "" {
			meta.LineLabel = strings.TrimSpace(g.LineLabel)
		}
		if strings.TrimSpace(meta.LineLabel) == "" && fromHdr != "" {
			if xl := conferenceEndpointLineLabel(cfg, g, fromHdr); xl != "" {
				meta.LineLabel = xl
			}
		}
	} else {
		br, ok := bridgeByID(cfg, bridgeID)
		if !ok {
			log.Printf("SIPREC skip bridge_id=%s: bridge not found in config", bridgeID)
			return
		}
		if !bridgeRecordingEnabled(br) && (user == nil || !user.RecordingOptIn) {
			log.Printf("SIPREC skip bridge_id=%s: bridge recording off and participant recording opt-in false or unknown user", bridgeID)
			return
		}
		meta.BridgeID = bridgeID
		meta.BridgeName = strings.TrimSpace(br.Name)
		if strings.TrimSpace(meta.LineLabel) == "" && fromHdr != "" {
			meta.LineLabel = bridgeLineLabelForFrom(cfg, bridgeID, fromHdr)
		}
	}
	applyLineLabelFromUserDevices(&meta, user)
	xml, err := siprec.BuildMetadataXML(meta)
	if err != nil {
		log.Printf("SIPREC metadata: %v", err)
		return
	}
	inviteURI := pickRecordingInviteURI(trunk)
	logicalKey := "bridge:" + bridgeID + ":" + bridgeKey
	if err := s.emitSIPRECInvite(logicalKey, inviteURI, xml); err != nil {
		log.Printf("SIPREC start bridge: %v", err)
	}
}

// startSIPRECForConferenceFanoutEarly registers the inbound caller on the synthetic ARD bridge map
// and forks SIPREC as soon as the conference fanout session is created — before far-end 2xx or ACK.
// Duplicate tryStartSIPREC from the winner path is deduped by emitSIPRECInvite.
func (s *Server) startSIPRECForConferenceFanoutEarly(sessKey, gid string, inbound *Message, remote *net.UDPAddr, contactURI string) {
	gid = strings.TrimSpace(gid)
	if s == nil || gid == "" || inbound == nil {
		return
	}
	cfg := s.router.CurrentConfig()
	g, ok := conferenceGroupByID(cfg, gid)
	if !ok {
		return
	}
	fromHdr := inbound.Header("from")
	uid, disp := conferenceInviteParticipantLabels(cfg, g, fromHdr)
	allowSIPREC := conferenceGroupRecordingEnabled(g)
	if !allowSIPREC && uid != "" {
		for i := range cfg.Spec.Users {
			u := &cfg.Spec.Users[i]
			if u.ID == uid && u.RecordingOptIn {
				allowSIPREC = true
				break
			}
		}
	}
	if !allowSIPREC {
		return
	}
	toWithTag, toTag := ensureToTagWithValue(inbound.Header("to"))
	ardBridgeID := syntheticARDBridgeID(gid)
	s.sessionMu.Lock()
	m := s.bridgeCalls[ardBridgeID]
	if m == nil {
		m = make(map[string]*bridgeCall)
		s.bridgeCalls[ardBridgeID] = m
	}
	m[sessKey] = &bridgeCall{
		bridgeID:        ardBridgeID,
		callID:          strings.TrimSpace(inbound.Header("call-id")),
		fromTag:         extractTag(fromHdr),
		toTag:           toTag,
		fromHeader:      fromHdr,
		toHeader:        toWithTag,
		contactURI:      contactURI,
		remote:          remote,
		createdAt:       time.Now().UTC(),
		userID:          uid,
		userDisplayName: disp,
		lineLabel:       strings.TrimSpace(g.LineLabel),
	}
	s.sessionMu.Unlock()
	log.Printf("SIPREC early start (inbound leg joined, not waiting for far-end 2xx) conference group_id=%s session=%s", gid, sessKey)
	go s.tryStartSIPRECForBridge(ardBridgeID, sessKey)
}

// startSIPRECForBridgeFanoutEarly registers the inbound caller and forks SIPREC when bridge fanout
// begins, before any callee answers.
func (s *Server) startSIPRECForBridgeFanoutEarly(sessKey, bridgeID string, inbound *Message, remote *net.UDPAddr, contactURI string) {
	bridgeID = strings.TrimSpace(bridgeID)
	if s == nil || bridgeID == "" || inbound == nil {
		return
	}
	cfg := s.router.CurrentConfig()
	if _, ok := bridgeByID(cfg, bridgeID); !ok {
		return
	}
	fromHdr := inbound.Header("from")
	toWithTag, toTag := ensureToTagWithValue(inbound.Header("to"))
	lineLbl := bridgeLineLabelForFrom(cfg, bridgeID, fromHdr)
	s.sessionMu.Lock()
	m := s.bridgeCalls[bridgeID]
	if m == nil {
		m = make(map[string]*bridgeCall)
		s.bridgeCalls[bridgeID] = m
	}
	m[sessKey] = &bridgeCall{
		bridgeID:   bridgeID,
		callID:     strings.TrimSpace(inbound.Header("call-id")),
		fromTag:    extractTag(fromHdr),
		toTag:      toTag,
		fromHeader: fromHdr,
		toHeader:   toWithTag,
		contactURI: contactURI,
		remote:     remote,
		createdAt:  time.Now().UTC(),
		lineLabel:  lineLbl,
	}
	s.sessionMu.Unlock()
	log.Printf("SIPREC early start (inbound leg joined, not waiting for far-end 2xx) bridge_id=%s session=%s", bridgeID, sessKey)
	go s.tryStartSIPRECForBridge(bridgeID, sessKey)
}

func (s *Server) emitSIPRECInvite(logicalKey, requestURI, metadataXML string) error {
	if strings.TrimSpace(requestURI) == "" {
		return nil
	}
	s.mu.RLock()
	conn := s.udpConn
	s.mu.RUnlock()
	if conn == nil {
		log.Printf("SIPREC skip: SIP UDP listener not ready (udpConn nil) logical=%s", logicalKey)
		return nil
	}
	la := conn.LocalAddr().(*net.UDPAddr)
	localIP := advertisedIP(conn, nil)
	if strings.TrimSpace(s.cfg.AdvertiseAddr) != "" {
		localIP = strings.TrimSpace(s.cfg.AdvertiseAddr)
	} else if lip := net.ParseIP(localIP); lip != nil && (lip.IsLoopback() || isLikelyDockerBridgeIPv4(lip)) {
		log.Printf(
			"SIPREC SDP warning: advertised IP %s is not routable from the LAN for RTP. Set SIP_ADVERTISE_ADDR or sip.advertise_addr to this host's LAN IP (e.g. 192.168.1.41).",
			localIP,
		)
	}
	localPort := la.Port

	s.sessionMu.Lock()
	if s.siprecRecordings == nil {
		s.siprecRecordings = make(map[string]*fanoutLeg)
	}
	if _, exists := s.siprecRecordings[logicalKey]; exists {
		s.sessionMu.Unlock()
		log.Printf("SIPREC skip: already active for logical=%s", logicalKey)
		return nil
	}
	s.sessionMu.Unlock()

	sessionKey := "siprec|" + logicalKey
	sdp := buildSiprecInviteSDP(localIP, siprecInviteRTPBase())
	extra := OutboundExtraHeaders{SessionTimer: s.cfg.SessionTimerEnabled}
	out, err := BuildOutboundInviteSIPREC(requestURI, localIP, localPort, ViaUDP, sdp, metadataXML, extra)
	if err != nil {
		return err
	}
	leg := &fanoutLeg{
		sessionKey:   sessionKey,
		targetURI:    out.Target,
		callID:       out.CallID,
		branch:       out.Branch,
		fromTag:      out.FromTag,
		viaTransport: ViaUDP,
		localViaHost: localIP,
		localViaPort: localPort,
	}

	s.sessionMu.Lock()
	if s.siprecRecordings == nil {
		s.siprecRecordings = make(map[string]*fanoutLeg)
	}
	if _, exists := s.siprecRecordings[logicalKey]; exists {
		s.sessionMu.Unlock()
		log.Printf("SIPREC skip: race duplicate logical=%s", logicalKey)
		return nil
	}
	s.legsByCallID[out.CallID] = leg
	s.siprecRecordings[logicalKey] = leg
	s.sessionMu.Unlock()

	dst := s.outboundDestForTarget(out.Target, nil)
	if dst == nil {
		log.Printf("SIPREC skip: cannot resolve UDP destination for target_uri=%q (check host/port/DNS; IPv6 host literals need brackets in sip: URI)", out.Target)
		s.sessionMu.Lock()
		delete(s.legsByCallID, out.CallID)
		delete(s.siprecRecordings, logicalKey)
		s.sessionMu.Unlock()
		return nil
	}
	if _, err := conn.WriteToUDP(out.Bytes, dst); err != nil {
		s.sessionMu.Lock()
		delete(s.legsByCallID, out.CallID)
		delete(s.siprecRecordings, logicalKey)
		s.sessionMu.Unlock()
		return err
	}
	log.Printf(
		"SIPREC INVITE sent logical=%s call_id=%s uri=%s (PCAP=rtpengine media; SIPREC UI requires drachtio to deliver this INVITE to SIPRec Node)",
		logicalKey, out.CallID, requestURI,
	)
	return nil
}

func (s *Server) stopSIPRECRecording(logicalKey string) {
	if s == nil {
		return
	}
	s.sessionMu.Lock()
	if s.siprecRecordings == nil {
		s.sessionMu.Unlock()
		return
	}
	leg := s.siprecRecordings[logicalKey]
	if leg == nil {
		s.sessionMu.Unlock()
		return
	}
	delete(s.siprecRecordings, logicalKey)
	callID := leg.callID
	hasTo := strings.TrimSpace(leg.toHeader) != ""
	s.sessionMu.Unlock()

	s.clearSIPRECMediaForwardingByKey(logicalKey)

	if leg.outboundConn != nil {
		// TLS recording not wired in this path
		_ = leg.outboundConn.Close()
	}
	s.mu.RLock()
	conn := s.udpConn
	s.mu.RUnlock()
	if conn == nil {
		s.sessionMu.Lock()
		delete(s.legsByCallID, callID)
		s.sessionMu.Unlock()
		return
	}

	li, lp := s.localViaForLeg(leg)
	if !hasTo {
		cancel := BuildOutboundCancel(leg.targetURI, li, lp, leg.callID, leg.branch, leg.fromTag, leg.viaTransport)
		_ = s.writeLegOutbound(leg, cancel)
	} else {
		reqURI := leg.siprecByeURI
		if strings.TrimSpace(reqURI) == "" {
			reqURI = leg.targetURI
		}
		toHdr := leg.toHeader
		if strings.TrimSpace(toHdr) == "" {
			toHdr = "<" + leg.targetURI + ">"
		}
		branch := "z9hG4bK" + RandHex(12)
		bye := BuildOutboundByeForDialog(reqURI, li, lp, leg.callID, branch, leg.fromTag, toHdr, leg.viaTransport)
		_ = s.writeLegOutbound(leg, bye)
	}
	s.sessionMu.Lock()
	delete(s.legsByCallID, callID)
	s.sessionMu.Unlock()
	log.Printf("SIPREC teardown logical=%s call_id=%s", logicalKey, callID)
}
