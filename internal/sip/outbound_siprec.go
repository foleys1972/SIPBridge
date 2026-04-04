package sip

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// BuildOutboundInviteSIPREC sends a multipart INVITE toward a SIPREC recording server (RFC 7866-style).
// sdp is typically application/sdp; metadataXML is participant/session metadata (application/xml).
func BuildOutboundInviteSIPREC(targetURI, viaHost string, viaPort int, viaTx SIPViaTransport, sdp string, metadataXML string, extra OutboundExtraHeaders) (OutboundInvite, error) {
	callID := RandHex(16) + "@" + viaHost
	branch := "z9hG4bK" + RandHex(12)
	fromTag := RandHex(8)

	contactParams := "transport=udp"
	if viaTx == ViaTLS {
		contactParams = "transport=tls"
	} else if viaTx == ViaTCP {
		contactParams = "transport=tcp"
	}

	boundary := "siprec_" + RandHex(10)
	var body strings.Builder
	fmt.Fprintf(&body, "--%s\r\n", boundary)
	body.WriteString("Content-Type: application/sdp\r\n")
	body.WriteString("Content-Disposition: session\r\n\r\n")
	body.WriteString(sdp)
	body.WriteString("\r\n")
	fmt.Fprintf(&body, "--%s\r\n", boundary)
	body.WriteString("Content-Type: application/xml; charset=UTF-8\r\n")
	body.WriteString("Content-Disposition: recording-session\r\n\r\n")
	body.WriteString(metadataXML)
	body.WriteString("\r\n")
	fmt.Fprintf(&body, "--%s--\r\n", boundary)
	bodyStr := body.String()

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
	fmt.Fprintf(b, "Content-Type: multipart/mixed; boundary=%s\r\n", boundary)
	b.WriteString("Content-Length: ")
	b.WriteString(strconv.Itoa(len(bodyStr)))
	b.WriteString("\r\n\r\n")
	b.WriteString(bodyStr)

	_ = time.Now()
	return OutboundInvite{
		Bytes:   []byte(b.String()),
		Target:  targetURI,
		CallID:  callID,
		Branch:  branch,
		FromTag: fromTag,
	}, nil
}

// BuildOutboundByeForDialog sends BYE on an established outbound dialog (Request-URI + full To header from 200 OK).
func BuildOutboundByeForDialog(requestURI string, localIP string, localPort int, callID, branch, fromTag string, toHeader string, tx SIPViaTransport) []byte {
	b := &strings.Builder{}
	fmt.Fprintf(b, "BYE %s SIP/2.0\r\n", requestURI)
	fmt.Fprintf(b, "Via: SIP/2.0/%s %s:%d;branch=%s\r\n", viaName(tx), localIP, localPort, branch)
	b.WriteString("Max-Forwards: 70\r\n")
	fmt.Fprintf(b, "From: <sip:sipbridge@%s>;tag=%s\r\n", localIP, fromTag)
	toHeader = strings.TrimSpace(toHeader)
	if toHeader != "" {
		if strings.HasPrefix(strings.ToLower(toHeader), "to:") {
			b.WriteString(toHeader)
			if !strings.HasSuffix(toHeader, "\r\n") {
				b.WriteString("\r\n")
			}
		} else {
			fmt.Fprintf(b, "To: %s\r\n", toHeader)
		}
	}
	fmt.Fprintf(b, "Call-ID: %s\r\n", callID)
	b.WriteString("CSeq: 2 BYE\r\n")
	b.WriteString("Content-Length: 0\r\n\r\n")
	return []byte(b.String())
}
