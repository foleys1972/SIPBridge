package sip

import (
	"net"
	"strconv"
	"strings"
)

type sdpAudioOffer struct {
	Addr             net.IP
	Port             int
	Payloads         []int
	HasPCMU          bool
	HasPCMA          bool
	TelephoneEventPT int
	RawConnIP        string
	RawConnLine      string
}

func parseSDPAudioOffer(body string, fallbackIP net.IP) (sdpAudioOffer, bool) {
	b := strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(b, "\n")
	o := sdpAudioOffer{Addr: append(net.IP(nil), fallbackIP...)}

	var connIP net.IP
	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		if strings.HasPrefix(l, "c=") {
			// c=IN IP4 1.2.3.4
			parts := strings.Fields(strings.TrimPrefix(l, "c="))
			if len(parts) >= 3 {
				o.RawConnLine = l
				o.RawConnIP = parts[2]
				if ip := net.ParseIP(parts[2]); ip != nil {
					connIP = ip
				}
			}
		}
	}
	if connIP == nil {
		connIP = sdpOriginUnicastIP(lines)
	}
	if connIP == nil && fallbackIP != nil {
		connIP = fallbackIP
	}
	if connIP != nil {
		o.Addr = connIP
	}

	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		if strings.HasPrefix(l, "m=audio ") {
			parts := strings.Fields(strings.TrimPrefix(l, "m="))
			// audio <port> RTP/AVP <pt...>
			if len(parts) < 4 {
				continue
			}
			p, err := strconv.Atoi(parts[1])
			if err != nil || p <= 0 {
				continue
			}
			o.Port = p
			for _, ptS := range parts[3:] {
				pt, err := strconv.Atoi(ptS)
				if err != nil {
					continue
				}
				o.Payloads = append(o.Payloads, pt)
				switch pt {
				case 0:
					o.HasPCMU = true
				case 8:
					o.HasPCMA = true
				}
			}
			break
		}
	}

	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		lower := strings.ToLower(l)
		if !strings.HasPrefix(lower, "a=rtpmap:") {
			continue
		}
		// a=rtpmap:<pt> <encoding>/<rate>
		rest := strings.TrimSpace(l[len("a=rtpmap:"):])
		parts := strings.Fields(rest)
		if len(parts) < 2 {
			continue
		}
		pt, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		enc := strings.ToLower(strings.TrimSpace(parts[1]))
		if strings.HasPrefix(enc, "telephone-event/") {
			o.TelephoneEventPT = pt
		}
	}

	if o.Port == 0 || o.Addr == nil {
		return sdpAudioOffer{}, false
	}
	return o, true
}

// pickIVRAudioPayloadType returns G.711 payload type (0=PCMU, 8=PCMA) using the first
// matching codec in m=audio payload order.
func pickIVRAudioPayloadType(o sdpAudioOffer) (uint8, bool) {
	if !o.HasPCMU && !o.HasPCMA {
		return 0, false
	}
	for _, pt := range o.Payloads {
		switch pt {
		case 0:
			if o.HasPCMU {
				return 0, true
			}
		case 8:
			if o.HasPCMA {
				return 8, true
			}
		}
	}
	if o.HasPCMU {
		return 0, true
	}
	return 8, true
}

// sdpOriginUnicastIP returns the address from the first o= line (RFC 4566 unicast-address)
// when session/media c= is absent — some UAs omit c= and rely on o= for the default address.
func sdpOriginUnicastIP(lines []string) net.IP {
	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		if !strings.HasPrefix(l, "o=") {
			continue
		}
		parts := strings.Fields(strings.TrimPrefix(l, "o="))
		if len(parts) < 6 {
			continue
		}
		// o=<username> <sess-id> <sess-version> <nettype> <addrtype> <unicast-address>
		if ip := net.ParseIP(strings.Trim(parts[5], "[]")); ip != nil {
			return ip
		}
	}
	return nil
}

// parseMultipleAudioRTPDestinations returns UDP destinations for each m=audio line in order.
// Session-level c= applies until a later c= overrides the address for subsequent media sections.
func parseMultipleAudioRTPDestinations(body string) ([]*net.UDPAddr, bool) {
	b := strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(b, "\n")
	var defaultIP net.IP
	var currentIP net.IP
	var out []*net.UDPAddr

	for _, ln := range lines {
		l := strings.TrimSpace(ln)
		if strings.HasPrefix(l, "c=") {
			parts := strings.Fields(strings.TrimPrefix(l, "c="))
			if len(parts) >= 3 {
				if ip := net.ParseIP(parts[2]); ip != nil {
					currentIP = ip
					if defaultIP == nil {
						defaultIP = ip
					}
				}
			}
			continue
		}
		if !strings.HasPrefix(l, "m=audio ") {
			continue
		}
		parts := strings.Fields(strings.TrimPrefix(l, "m="))
		if len(parts) < 4 {
			continue
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil || port <= 0 {
			continue
		}
		ip := currentIP
		if ip == nil {
			ip = defaultIP
		}
		if ip == nil {
			ip = sdpOriginUnicastIP(lines)
		}
		if ip == nil {
			continue
		}
		out = append(out, &net.UDPAddr{IP: ip, Port: port})
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func buildIVRSDPAnswer(ip string, rtpPort int, telephoneEventPT int) string {
	// Always advertise telephone-event for SBC interop (RFC 4733). Default to PT 101 when absent.
	if telephoneEventPT <= 0 || telephoneEventPT >= 128 {
		telephoneEventPT = 101
	}
	family := "IP4"
	if parsed := net.ParseIP(strings.TrimSpace(ip)); parsed != nil {
		if parsed.To4() == nil {
			family = "IP6"
		}
	}
	var b strings.Builder
	b.WriteString("v=0\r\n")
	b.WriteString("o=sipbridge 0 0 IN ")
	b.WriteString(family)
	b.WriteString(" ")
	b.WriteString(ip)
	b.WriteString("\r\n")
	b.WriteString("s=SIPBridge\r\n")
	b.WriteString("c=IN ")
	b.WriteString(family)
	b.WriteString(" ")
	b.WriteString(ip)
	b.WriteString("\r\n")
	b.WriteString("t=0 0\r\n")
	b.WriteString("m=audio ")
	b.WriteString(strconv.Itoa(rtpPort))
	b.WriteString(" RTP/AVP 0")
	if telephoneEventPT > 0 && telephoneEventPT < 128 {
		b.WriteString(" ")
		b.WriteString(strconv.Itoa(telephoneEventPT))
	}
	b.WriteString("\r\n")
	b.WriteString("a=rtpmap:0 PCMU/8000\r\n")
	if telephoneEventPT > 0 && telephoneEventPT < 128 {
		fmtp := "0-16"
		b.WriteString("a=rtpmap:")
		b.WriteString(strconv.Itoa(telephoneEventPT))
		b.WriteString(" telephone-event/8000\r\n")
		b.WriteString("a=fmtp:")
		b.WriteString(strconv.Itoa(telephoneEventPT))
		b.WriteString(" ")
		b.WriteString(fmtp)
		b.WriteString("\r\n")
	}
	b.WriteString("a=sendrecv\r\n")
	return b.String()
}

// buildBridgeFanoutSDPAnswer is like buildIVRSDPAnswer but advertises PCMU (pt 0) or PCMA (pt 8) to match the inbound offer.
func buildBridgeFanoutSDPAnswer(ip string, rtpPort int, telephoneEventPT int, audioPT uint8) string {
	// Always advertise telephone-event for SBC interop (RFC 4733). Default to PT 101 when absent.
	if telephoneEventPT <= 0 || telephoneEventPT >= 128 {
		telephoneEventPT = 101
	}
	family := "IP4"
	if parsed := net.ParseIP(strings.TrimSpace(ip)); parsed != nil {
		if parsed.To4() == nil {
			family = "IP6"
		}
	}
	pt := int(audioPT)
	if pt != 0 && pt != 8 {
		pt = 0
	}
	var b strings.Builder
	b.WriteString("v=0\r\n")
	b.WriteString("o=sipbridge 0 0 IN ")
	b.WriteString(family)
	b.WriteString(" ")
	b.WriteString(ip)
	b.WriteString("\r\n")
	b.WriteString("s=SIPBridge\r\n")
	b.WriteString("c=IN ")
	b.WriteString(family)
	b.WriteString(" ")
	b.WriteString(ip)
	b.WriteString("\r\n")
	b.WriteString("t=0 0\r\n")
	b.WriteString("m=audio ")
	b.WriteString(strconv.Itoa(rtpPort))
	b.WriteString(" RTP/AVP ")
	b.WriteString(strconv.Itoa(pt))
	if telephoneEventPT > 0 && telephoneEventPT < 128 {
		b.WriteString(" ")
		b.WriteString(strconv.Itoa(telephoneEventPT))
	}
	b.WriteString("\r\n")
	if pt == 8 {
		b.WriteString("a=rtpmap:8 PCMA/8000\r\n")
	} else {
		b.WriteString("a=rtpmap:0 PCMU/8000\r\n")
	}
	if telephoneEventPT > 0 && telephoneEventPT < 128 {
		fmtp := "0-16"
		b.WriteString("a=rtpmap:")
		b.WriteString(strconv.Itoa(telephoneEventPT))
		b.WriteString(" telephone-event/8000\r\n")
		b.WriteString("a=fmtp:")
		b.WriteString(strconv.Itoa(telephoneEventPT))
		b.WriteString(" ")
		b.WriteString(fmtp)
		b.WriteString("\r\n")
	}
	b.WriteString("a=sendrecv\r\n")
	return b.String()
}
