package sip

import (
	"log"
	"net"
	"strings"

	"sipbridge/internal/config"
	"sipbridge/internal/siprec"
)

func pickRecordingInviteURI(t config.RecordingTrunkEntry) string {
	if u := strings.TrimSpace(t.RecordingTrunkSIPURI); u != "" {
		return u
	}
	return strings.TrimSpace(t.RecorderSIPURI)
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
	if !ok || !g.RecordingEnabled {
		return
	}
	siprecSpec := rec.SIPREC
	trunk, ok := config.SelectRecordingTrunkForRegion(siprecSpec, sess.preferredRegion)
	if !ok || strings.TrimSpace(trunk.RecorderSIPURI) == "" {
		return
	}
	// PIN dial-in: require a matching user with recording opt-in.
	pin := strings.TrimSpace(sess.participantID)
	var user *config.User
	for i := range root.Spec.Users {
		u := &root.Spec.Users[i]
		if strings.TrimSpace(u.ParticipantID) == pin {
			user = u
			break
		}
	}
	if pin != "" {
		if user == nil {
			log.Printf("SIPREC skip: no config user for PIN dial-in")
			return
		}
		if !user.RecordingOptIn {
			log.Printf("SIPREC skip: user %q has recording_opt_in=false", user.ID)
			return
		}
	}

	meta := siprec.ParticipantRecordingMeta{
		ConferenceGroupID: gid,
		DialIn:              pin != "",
	}
	if user != nil {
		meta.EmployeeID = user.ID
		meta.DisplayName = user.DisplayName
	}
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
	if rec == nil || !rec.GlobalEnabled || rec.SIPREC == nil || !rec.SIPREC.Enabled {
		return
	}
	siprecSpec := rec.SIPREC
	trunk, ok := config.SelectRecordingTrunkForRegion(siprecSpec, "")
	if !ok || strings.TrimSpace(trunk.RecorderSIPURI) == "" {
		return
	}
	meta := siprec.ParticipantRecordingMeta{}
	if strings.HasPrefix(bridgeID, ardGroupBridgePrefix) {
		gid := strings.TrimPrefix(bridgeID, ardGroupBridgePrefix)
		g, ok := conferenceGroupByID(cfg, gid)
		if !ok || !g.RecordingEnabled {
			return
		}
		meta.ConferenceGroupID = gid
	} else {
		meta.BridgeID = bridgeID
	}
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

func (s *Server) emitSIPRECInvite(logicalKey, requestURI, metadataXML string) error {
	if strings.TrimSpace(requestURI) == "" {
		return nil
	}
	s.mu.RLock()
	conn := s.udpConn
	s.mu.RUnlock()
	if conn == nil {
		return nil
	}
	la := conn.LocalAddr().(*net.UDPAddr)
	localIP := advertisedIP(conn, nil)
	if strings.TrimSpace(s.cfg.AdvertiseAddr) != "" {
		localIP = strings.TrimSpace(s.cfg.AdvertiseAddr)
	}
	localPort := la.Port

	s.sessionMu.Lock()
	if s.siprecRecordings == nil {
		s.siprecRecordings = make(map[string]*fanoutLeg)
	}
	if _, exists := s.siprecRecordings[logicalKey]; exists {
		s.sessionMu.Unlock()
		return nil
	}
	s.sessionMu.Unlock()

	sessionKey := "siprec|" + logicalKey
	sdp := buildMinimalSDP(localIP, localPort)
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
		return nil
	}
	s.legsByCallID[out.CallID] = leg
	s.siprecRecordings[logicalKey] = leg
	s.sessionMu.Unlock()

	dst := s.outboundDestForTarget(out.Target, nil)
	if dst == nil {
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
	log.Printf("SIPREC INVITE sent logical=%s call_id=%s uri=%s", logicalKey, out.CallID, requestURI)
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
