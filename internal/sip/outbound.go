package sip

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type OutboundInvite struct {
	Bytes   []byte
	Target  string
	CallID  string
	Branch  string
	FromTag string
}

// SIPViaTransport is the token used in Via (UDP, TCP, TLS).
type SIPViaTransport string

const (
	ViaUDP SIPViaTransport = "UDP"
	ViaTLS SIPViaTransport = "TLS"
	ViaTCP SIPViaTransport = "TCP"
)

func BuildOutboundInvite(targetURI, localIP string, localPort int) (OutboundInvite, error) {
	sdp := buildMinimalSDP(localIP, localPort)
	return BuildOutboundInviteWithSDP(targetURI, localIP, localPort, sdp)
}

func BuildOutboundInviteWithSDP(targetURI, localIP string, localPort int, sdp string) (OutboundInvite, error) {
	return BuildOutboundInviteFull(targetURI, localIP, localPort, ViaUDP, sdp, OutboundExtraHeaders{})
}

// OutboundExtraHeaders optional interop fields for Oracle / AudioCodes SBCs.
type OutboundExtraHeaders struct {
	SessionTimer bool // Min-SE + Session-Expires (common SBC requirement)
}

func BuildOutboundInviteFull(targetURI, viaHost string, viaPort int, viaTx SIPViaTransport, sdp string, extra OutboundExtraHeaders) (OutboundInvite, error) {
	callID := RandHex(16) + "@" + viaHost
	branch := "z9hG4bK" + RandHex(12)
	fromTag := RandHex(8)

	contactParams := "transport=udp"
	if viaTx == ViaTLS {
		contactParams = "transport=tls"
	} else if viaTx == ViaTCP {
		contactParams = "transport=tcp"
	}

	b := &strings.Builder{}
	fmt.Fprintf(b, "INVITE %s SIP/2.0\r\n", targetURI)
	fmt.Fprintf(b, "Via: SIP/2.0/%s %s:%d;branch=%s;rport\r\n", string(viaTx), viaHost, viaPort, branch)
	b.WriteString("Max-Forwards: 70\r\n")
	fmt.Fprintf(b, "From: <sip:sipbridge@%s>;tag=%s\r\n", viaHost, fromTag)
	fmt.Fprintf(b, "To: <%s>\r\n", targetURI)
	fmt.Fprintf(b, "Call-ID: %s\r\n", callID)
	b.WriteString("CSeq: 1 INVITE\r\n")
	fmt.Fprintf(b, "Contact: <sip:sipbridge@%s:%d;%s>\r\n", viaHost, viaPort, contactParams)
	b.WriteString("Allow: INVITE, ACK, BYE, CANCEL, OPTIONS, INFO, REFER, UPDATE\r\n")
	b.WriteString("Supported: replaces, norefersub, timer\r\n")
	if extra.SessionTimer {
		b.WriteString("Min-SE: 90\r\n")
		b.WriteString("Session-Expires: 1800;refresher=uac\r\n")
	}
	b.WriteString("Content-Type: application/sdp\r\n")
	b.WriteString("Content-Length: ")
	b.WriteString(strconv.Itoa(len(sdp)))
	b.WriteString("\r\n\r\n")
	b.WriteString(sdp)

	_ = time.Now()
	return OutboundInvite{
		Bytes:   []byte(b.String()),
		Target:  targetURI,
		CallID:  callID,
		Branch:  branch,
		FromTag: fromTag,
	}, nil
}

func RandHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randHex(nBytes int) string { return RandHex(nBytes) }

func viaName(tx SIPViaTransport) string {
	if tx == "" {
		return string(ViaUDP)
	}
	return string(tx)
}

// BuildOutboundAck sends ACK for 200 OK on outbound leg (transport must match deployment).
func BuildOutboundAck(targetURI, localIP string, localPort int, callID, fromTag, toTag string, tx SIPViaTransport) []byte {
	branch := "z9hG4bK" + RandHex(12)
	b := &strings.Builder{}
	fmt.Fprintf(b, "ACK %s SIP/2.0\r\n", targetURI)
	fmt.Fprintf(b, "Via: SIP/2.0/%s %s:%d;branch=%s\r\n", viaName(tx), localIP, localPort, branch)
	b.WriteString("Max-Forwards: 70\r\n")
	fmt.Fprintf(b, "From: <sip:sipbridge@%s>;tag=%s\r\n", localIP, fromTag)
	if strings.TrimSpace(toTag) != "" {
		fmt.Fprintf(b, "To: <%s>;tag=%s\r\n", targetURI, toTag)
	} else {
		fmt.Fprintf(b, "To: <%s>\r\n", targetURI)
	}
	fmt.Fprintf(b, "Call-ID: %s\r\n", callID)
	b.WriteString("CSeq: 1 ACK\r\n")
	b.WriteString("Content-Length: 0\r\n\r\n")
	return []byte(b.String())
}

// BuildOutboundCancel must use the same Via branch as the INVITE (passed as branch).
func BuildOutboundCancel(targetURI, localIP string, localPort int, callID, branch, fromTag string, tx SIPViaTransport) []byte {
	b := &strings.Builder{}
	fmt.Fprintf(b, "CANCEL %s SIP/2.0\r\n", targetURI)
	fmt.Fprintf(b, "Via: SIP/2.0/%s %s:%d;branch=%s\r\n", viaName(tx), localIP, localPort, branch)
	b.WriteString("Max-Forwards: 70\r\n")
	fmt.Fprintf(b, "From: <sip:sipbridge@%s>;tag=%s\r\n", localIP, fromTag)
	fmt.Fprintf(b, "To: <%s>\r\n", targetURI)
	fmt.Fprintf(b, "Call-ID: %s\r\n", callID)
	b.WriteString("CSeq: 1 CANCEL\r\n")
	b.WriteString("Content-Length: 0\r\n\r\n")
	return []byte(b.String())
}

func BuildOutboundBye(targetURI, localIP string, localPort int, callID, branch, fromTag string, tx SIPViaTransport) []byte {
	b := &strings.Builder{}
	fmt.Fprintf(b, "BYE %s SIP/2.0\r\n", targetURI)
	fmt.Fprintf(b, "Via: SIP/2.0/%s %s:%d;branch=%s\r\n", viaName(tx), localIP, localPort, branch)
	b.WriteString("Max-Forwards: 70\r\n")
	fmt.Fprintf(b, "From: <sip:sipbridge@%s>;tag=%s\r\n", localIP, fromTag)
	fmt.Fprintf(b, "To: <%s>\r\n", targetURI)
	fmt.Fprintf(b, "Call-ID: %s\r\n", callID)
	b.WriteString("CSeq: 2 BYE\r\n")
	b.WriteString("Content-Length: 0\r\n\r\n")
	return []byte(b.String())
}
