package sip

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"sipbridge/internal/config"
)

// ardGroupBridgePrefix is the synthetic bridge ID prefix for ARD conference group legs
// stored in bridgeCalls so established lines can be counted and BYE/RE-INVITE behave like bridge rooms.
const ardGroupBridgePrefix = "ardGroup:"

func isARDGroup(g config.ConferenceGroup) bool {
	return strings.EqualFold(strings.TrimSpace(g.Type), "ard")
}

func syntheticARDBridgeID(groupID string) string {
	return ardGroupBridgePrefix + strings.TrimSpace(groupID)
}

func (s *Server) ardEstablishedParticipantCount(groupID string) int {
	if s == nil {
		return 0
	}
	gid := strings.TrimSpace(groupID)
	if gid == "" {
		return 0
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	m := s.bridgeCalls[syntheticARDBridgeID(gid)]
	if m == nil {
		return 0
	}
	return len(m)
}

// answerARDJoinInbound answers with 200 OK without fanout when the ARD line already has participants.
func (s *Server) answerARDJoinInbound(msg *Message, conn *net.UDPConn, remote *net.UDPAddr, advertiseIP string, localPort int, sessKey, groupID string, fromHdr, contactURI string) {
	toWithTag, toTag := ensureToTagWithValue(msg.Header("to"))
	callID := strings.TrimSpace(msg.Header("call-id"))
	fromTag := extractTag(fromHdr)
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
			rtpSess = newRTPSession(pc, &net.UDPAddr{IP: remoteRTPIP, Port: offer.Port}, 0, telPT, nil)
			rtpSess.StartReceiver()
			rtpPort = rtpSess.LocalPort()
		}
	}
	if rtpPort == 0 {
		rtpPort = localPort
	}
	var sdp string
	if rtpSess != nil {
		sdp = buildIVRSDPAnswer(advertiseIP, rtpPort, offer.TelephoneEventPT)
		log.Printf("ARD join RTP local_rtp_port=%d remote_rtp=%s group_id=%s", rtpPort, rtpSess.remote.String(), groupID)
	} else {
		sdp = buildMinimalSDP(advertiseIP, localPort)
		log.Printf("ARD join no PCMU offer; minimal SDP group_id=%s", groupID)
	}
	extra := map[string]string{
		"Contact":      fmt.Sprintf("<sip:sipbridge@%s;transport=udp>", net.JoinHostPort(advertiseIP, strconv.Itoa(localPort))),
		"Content-Type": "application/sdp",
		"Allow":        "INVITE, ACK, BYE, CANCEL, OPTIONS, INFO, REFER",
		"Supported":    "replaces, norefersub",
		"To":           toWithTag,
	}
	okResp, _ := BuildResponse(msg, 200, "OK", extra, []byte(sdp))
	if _, wErr := conn.WriteToUDP(okResp, remote); wErr != nil {
		log.Printf("SIP UDP tx error method=INVITE status=200 ARD join to=%s err=%v", remote.String(), wErr)
	} else {
		log.Printf("SIP INVITE ARD join 200 OK group_id=%s to=%s", groupID, remote.String())
	}

	bridgeID := syntheticARDBridgeID(groupID)
	s.sessionMu.Lock()
	m := s.bridgeCalls[bridgeID]
	if m == nil {
		m = make(map[string]*bridgeCall)
		s.bridgeCalls[bridgeID] = m
	}
	m[sessKey] = &bridgeCall{
		bridgeID:   bridgeID,
		callID:     callID,
		fromTag:    fromTag,
		toTag:      toTag,
		fromHeader: fromHdr,
		toHeader:   toWithTag,
		contactURI: contactURI,
		remote:     remote,
		createdAt:  time.Now().UTC(),
		rtp:        rtpSess,
	}
	s.sessionMu.Unlock()
	s.tryStartSIPRECForBridge(bridgeID, sessKey)
}
