package sip

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

func (s *Server) dialOutboundTLS() (*tls.Conn, error) {
	if s.tlsClient == nil {
		return nil, fmt.Errorf("TLS client not initialized (check SIP_OUTBOUND_* TLS env vars)")
	}
	addr := net.JoinHostPort(s.cfg.OutboundProxyAddr, strconv.Itoa(s.cfg.OutboundProxyPort))
	return tls.Dial("tcp", addr, s.tlsClient)
}

func dialOutboundTLSWithConfig(addr string, tc *tls.Config) (*tls.Conn, error) {
	if tc == nil {
		return nil, fmt.Errorf("TLS config is nil")
	}
	return tls.Dial("tcp", addr, tc)
}

func (s *Server) writeLegOutbound(leg *fanoutLeg, msg []byte) error {
	if leg == nil || s == nil {
		return fmt.Errorf("nil leg or server")
	}
	if leg.outboundConn != nil {
		return writeFull(leg.outboundConn, msg)
	}
	if s.udpConn == nil {
		return fmt.Errorf("no UDP conn")
	}
	if leg.outboundUDP != nil {
		_, err := s.udpConn.WriteToUDP(msg, leg.outboundUDP)
		return err
	}
	dst := s.outboundDestForTarget(leg.targetURI, nil)
	if dst == nil {
		return fmt.Errorf("unroutable target %s", leg.targetURI)
	}
	_, err := s.udpConn.WriteToUDP(msg, dst)
	return err
}

func (s *Server) localViaForLeg(leg *fanoutLeg) (host string, port int) {
	if leg != nil && leg.localViaHost != "" && leg.localViaPort > 0 {
		return leg.localViaHost, leg.localViaPort
	}
	if s.udpConn != nil {
		la := s.udpConn.LocalAddr().(*net.UDPAddr)
		return la.IP.String(), la.Port
	}
	return "127.0.0.1", 5060
}

func (s *Server) localPortForInbound(sess *fanoutSession) int {
	if sess == nil {
		return 5060
	}
	if sess.inboundBC.Conn != nil {
		if tcp, ok := sess.inboundBC.Conn.LocalAddr().(*net.TCPAddr); ok {
			return tcp.Port
		}
	}
	if sess.inboundBC.UDP != nil {
		if ua, ok := sess.inboundBC.UDP.LocalAddr().(*net.UDPAddr); ok {
			return ua.Port
		}
	}
	return 5060
}

func (s *Server) advertisedIPForInbound(sess *fanoutSession) string {
	if a := strings.TrimSpace(s.cfg.AdvertiseAddr); a != "" {
		return a
	}
	if sess == nil {
		return "127.0.0.1"
	}
	if sess.inboundBC.UDP != nil {
		return advertisedIP(sess.inboundBC.UDP, sess.inboundRemote)
	}
	return advertisedIPFromAddr(sess.inboundBC.LocalAddr(), sess.inboundRemote)
}

func advertisedIPFromAddr(la net.Addr, remote *net.UDPAddr) string {
	switch a := la.(type) {
	case *net.TCPAddr:
		if a.IP != nil && !a.IP.IsUnspecified() {
			return a.IP.String()
		}
	case *net.UDPAddr:
		if a.IP != nil && !a.IP.IsUnspecified() {
			return a.IP.String()
		}
	}
	if remote != nil && remote.IP != nil && !remote.IP.IsUnspecified() && !remote.IP.IsLoopback() {
		return remote.IP.String()
	}
	return "127.0.0.1"
}

func (s *Server) readTLSConnResponses(c net.Conn) {
	br := bufio.NewReader(c)
	for {
		raw, err := ReadNextSIPMessage(br)
		if err != nil {
			log.Printf("SIP TLS read end: %v", err)
			return
		}
		msg, err := ParseMessage(raw)
		if err != nil {
			log.Printf("SIP TLS parse: %v", err)
			continue
		}
		if !msg.IsRequest {
			s.handleResponse(msg)
		}
	}
}

// emitOutboundInvite sends one INVITE leg (UDP to target or TLS to configured SBC proxy).
func (s *Server) emitOutboundInvite(sessionKey string, targetURI string, sdp string, rtp *rtpSession, localIP string, localPort int) (*fanoutLeg, error) {
	extra := OutboundExtraHeaders{SessionTimer: s.cfg.SessionTimerEnabled}
	route := s.selectTrunkForTarget(targetURI)
	useTLS := s.cfg.OutboundTransport == "tls" && s.outboundDest(nil) != nil
	tlsClient := s.tlsClient
	tlsAddr := net.JoinHostPort(s.cfg.OutboundProxyAddr, strconv.Itoa(s.cfg.OutboundProxyPort))

	if route.trunk != nil {
		trTx := strings.ToLower(strings.TrimSpace(route.trunk.Transport))
		if trTx == "" {
			trTx = "udp"
		}
		if trTx == "tls" {
			useTLS = true
			tlsAddr = net.JoinHostPort(strings.TrimSpace(route.trunk.ProxyAddr), strconv.Itoa(route.trunk.ProxyPort))
			tc, err := NewTLSClientConfigValues(
				strings.TrimSpace(route.trunk.TLSServerName),
				strings.TrimSpace(route.trunk.TLSRootCAFile),
				strings.TrimSpace(route.trunk.TLSClientCertFile),
				strings.TrimSpace(route.trunk.TLSClientKeyFile),
				route.trunk.TLSInsecureSkipVerify,
			)
			if err != nil {
				return nil, err
			}
			tlsClient = tc
		} else {
			useTLS = false
		}
	}

	if useTLS {
		if tlsClient == nil {
			return nil, fmt.Errorf("outbound TLS not configured")
		}
		tlsConn, err := dialOutboundTLSWithConfig(tlsAddr, tlsClient)
		if err != nil {
			return nil, err
		}
		out, err := BuildOutboundInviteFull(targetURI, localIP, localPort, ViaTLS, sdp, extra)
		if err != nil {
			_ = tlsConn.Close()
			return nil, err
		}
		leg := &fanoutLeg{
			sessionKey:   sessionKey,
			targetURI:    out.Target,
			callID:       out.CallID,
			branch:       out.Branch,
			fromTag:      out.FromTag,
			outboundConn: tlsConn,
			viaTransport: ViaTLS,
			trunkID: func() string {
				if route.trunk != nil {
					return strings.TrimSpace(route.trunk.ID)
				}
				return ""
			}(),
			localViaHost: localIP,
			localViaPort: localPort,
			rtp:          rtp,
		}
		if err := writeFull(tlsConn, out.Bytes); err != nil {
			_ = tlsConn.Close()
			return nil, err
		}
		go s.readTLSConnResponses(tlsConn)
		return leg, nil
	}

	out, err := BuildOutboundInviteFull(targetURI, localIP, localPort, ViaUDP, sdp, extra)
	if err != nil {
		return nil, err
	}
	leg := &fanoutLeg{
		sessionKey:   sessionKey,
		targetURI:    out.Target,
		callID:       out.CallID,
		branch:       out.Branch,
		fromTag:      out.FromTag,
		viaTransport: ViaUDP,
		trunkID: func() string {
			if route.trunk != nil {
				return strings.TrimSpace(route.trunk.ID)
			}
			return ""
		}(),
		localViaHost: localIP,
		localViaPort: localPort,
		rtp:          rtp,
	}
	if s.udpConn == nil {
		return nil, fmt.Errorf("no UDP conn")
	}
	dst := route.udpDest
	if dst == nil {
		dst = s.outboundDestForTarget(out.Target, nil)
	}
	if dst == nil {
		return nil, fmt.Errorf("unroutable %s", out.Target)
	}
	leg.outboundUDP = dst
	if _, err := s.udpConn.WriteToUDP(out.Bytes, dst); err != nil {
		return nil, err
	}
	return leg, nil
}
