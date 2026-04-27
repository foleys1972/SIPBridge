package sip

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"sipbridge/internal/config"
)

// SIPRECProbeResult is returned by ProbeSIPREC (OPTIONS toward the configured recorder).
type SIPRECProbeResult struct {
	OK              bool   `json:"ok"`
	Reachable       bool   `json:"reachable"`
	TargetURI       string `json:"target_uri,omitempty"`
	Destination     string `json:"destination,omitempty"`
	Error           string `json:"error,omitempty"`
	SIPStatus       int    `json:"sip_status,omitempty"`
	Reason          string `json:"reason,omitempty"`
	ResponsePreview string `json:"response_preview,omitempty"`
	RoundtripMs     int64  `json:"roundtrip_ms,omitempty"`
	Step            string `json:"step,omitempty"`
	// Hint explains common non-2xx codes (e.g. 503 = drachtio up but no app on 9022).
	Hint string `json:"hint,omitempty"`
}

func buildOPTIONS(targetURI, localIP string, localPort int) []byte {
	callID := RandHex(16) + "@siprec-probe"
	branch := "z9hG4bK" + RandHex(12)
	fromTag := RandHex(8)
	var b strings.Builder
	fmt.Fprintf(&b, "OPTIONS %s SIP/2.0\r\n", targetURI)
	fmt.Fprintf(&b, "Via: SIP/2.0/UDP %s:%d;branch=%s;rport\r\n", localIP, localPort, branch)
	b.WriteString("Max-Forwards: 70\r\n")
	fmt.Fprintf(&b, "From: <sip:sipbridge@%s>;tag=%s\r\n", localIP, fromTag)
	fmt.Fprintf(&b, "To: <%s>\r\n", targetURI)
	fmt.Fprintf(&b, "Call-ID: %s\r\n", callID)
	b.WriteString("CSeq: 1 OPTIONS\r\n")
	b.WriteString("Content-Length: 0\r\n\r\n")
	return []byte(b.String())
}

// ProbeSIPREC sends SIP OPTIONS to the resolved recorder URI (same path as SIPREC INVITEs) over UDP
// using an ephemeral socket so the probe does not depend on the main SIP listener.
func (s *Server) ProbeSIPREC(ctx context.Context) SIPRECProbeResult {
	if s == nil || s.router == nil {
		return SIPRECProbeResult{Step: "init", Error: "sip server not ready"}
	}
	cfg := s.router.CurrentConfig()
	rec := cfg.Spec.Recording
	if rec == nil {
		return SIPRECProbeResult{Step: "config", Error: "spec.recording is not configured"}
	}
	if !rec.GlobalEnabled {
		return SIPRECProbeResult{Step: "config", Error: "global_enabled is false"}
	}
	if rec.SIPREC == nil || !rec.SIPREC.Enabled {
		return SIPRECProbeResult{Step: "config", Error: "siprec is disabled"}
	}
	trunk, ok := config.SelectRecordingTrunkForRegion(rec.SIPREC, "")
	if !ok || strings.TrimSpace(trunk.RecorderSIPURI) == "" {
		return SIPRECProbeResult{Step: "trunk", Error: "no recorder trunk or empty recorder_sip_uri"}
	}
	targetURI := pickRecordingInviteURI(trunk)
	if strings.TrimSpace(targetURI) == "" {
		return SIPRECProbeResult{Step: "trunk", Error: "could not resolve recorder SIP URI"}
	}
	res := SIPRECProbeResult{TargetURI: targetURI, Step: "resolve"}
	dst := s.outboundDestForTarget(targetURI, nil)
	if dst == nil {
		res.Error = "cannot resolve SIP UDP destination (check host, port, or DNS)"
		return res
	}
	res.Destination = dst.String()

	bindIP := net.ParseIP(s.cfg.BindAddr)
	if bindIP == nil {
		bindIP = net.IPv4zero
	}
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: bindIP, Port: 0})
	if err != nil {
		res.Step = "bind"
		res.Error = fmt.Sprintf("udp bind: %v", err)
		return res
	}
	defer pc.Close()

	localIP := advertisedIP(pc, nil)
	if strings.TrimSpace(s.cfg.AdvertiseAddr) != "" {
		localIP = strings.TrimSpace(s.cfg.AdvertiseAddr)
	}
	la := pc.LocalAddr().(*net.UDPAddr)
	localPort := la.Port

	payload := buildOPTIONS(targetURI, localIP, localPort)
	deadline := time.Now().Add(2 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = pc.SetWriteDeadline(deadline)
	t0 := time.Now()
	_, err = pc.WriteToUDP(payload, dst)
	if err != nil {
		res.Step = "send"
		res.Error = fmt.Sprintf("udp send: %v", err)
		return res
	}
	_ = pc.SetReadDeadline(deadline)
	buf := make([]byte, 8192)
	n, _, rerr := pc.ReadFromUDP(buf)
	res.RoundtripMs = time.Since(t0).Milliseconds()
	res.Step = "recv"
	if rerr != nil {
		res.Error = fmt.Sprintf("no SIP response (timeout or read error): %v", rerr)
		return res
	}
	raw := string(buf[:n])
	msg, perr := ParseMessage(buf[:n])
	if perr != nil || msg == nil {
		res.Error = "received non-SIP or malformed response"
		if n > 0 {
			trim := raw
			if len(trim) > 320 {
				trim = trim[:320] + "..."
			}
			res.ResponsePreview = trim
		}
		return res
	}
	if msg.IsRequest {
		res.Error = "received a SIP request instead of a response (unexpected)"
		return res
	}
	res.Reachable = true
	res.SIPStatus = msg.StatusCode
	res.Reason = msg.Reason
	preview := raw
	if len(preview) > 400 {
		preview = preview[:400] + "..."
	}
	res.ResponsePreview = preview
	// Treat 2xx as full success (typical for OPTIONS to drachtio).
	if msg.StatusCode >= 200 && msg.StatusCode < 300 {
		res.OK = true
		res.Step = "ok"
		return res
	}
	res.OK = false
	res.Error = fmt.Sprintf("SIP status %d %s (recorder responded but not with 2xx)", msg.StatusCode, strings.TrimSpace(msg.Reason))
	switch msg.StatusCode {
	case 503:
		res.Hint = "UDP path works, but the recorder returned 503 Service Unavailable. With drachtio + SIPREC this usually means no app is registered on the control plane: (1) SIPREC Node not running, or (2) drachtio.secret in config does not match the server (default cymru; Docker env DRACHTIO_SECRET / --secret). Check SIPREC logs for \"failed to authenticate\" and the Health row \"Drachtio app (this Node)\"."
	case 502:
		res.Hint = "Bad gateway — often an intermediate proxy cannot reach the upstream recorder."
	case 404:
		res.Hint = "Not found — Request-URI or user may not match what the recorder expects."
	case 405:
		res.Hint = "Method not allowed — some stacks reject OPTIONS; a full SIPREC INVITE test is still the real check."
	}
	return res
}
