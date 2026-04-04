package sip

import (
	"fmt"
	"strings"
	"time"
)

func BuildResponse(req *Message, statusCode int, reason string, extraHeaders map[string]string, body []byte) ([]byte, error) {
	if req == nil || !req.IsRequest {
		return nil, fmt.Errorf("response requires request message")
	}
	if reason == "" {
		reason = defaultReason(statusCode)
	}

	b := &strings.Builder{}
	fmt.Fprintf(b, "SIP/2.0 %d %s\r\n", statusCode, reason)

	// Echo required transaction identifiers.
	for _, via := range req.AllHeaders("via") {
		fmt.Fprintf(b, "Via: %s\r\n", via)
	}
	if from := req.Header("from"); from != "" {
		fmt.Fprintf(b, "From: %s\r\n", from)
	}
	if toOverride, ok := extraHeaders["To"]; ok && strings.TrimSpace(toOverride) != "" {
		fmt.Fprintf(b, "To: %s\r\n", strings.TrimSpace(toOverride))
		delete(extraHeaders, "To")
	} else if to := req.Header("to"); to != "" {
		if statusCode >= 200 {
			to = ensureToTag(to)
		}
		fmt.Fprintf(b, "To: %s\r\n", to)
	}
	if callID := req.Header("call-id"); callID != "" {
		fmt.Fprintf(b, "Call-ID: %s\r\n", callID)
	}
	if cseq := req.Header("cseq"); cseq != "" {
		fmt.Fprintf(b, "CSeq: %s\r\n", cseq)
	}

	fmt.Fprintf(b, "Server: SIPBridge/0.0.1\r\n")

	for k, v := range extraHeaders {
		if v == "" {
			continue
		}
		fmt.Fprintf(b, "%s: %s\r\n", k, v)
	}

	if body == nil {
		body = []byte{}
	}
	fmt.Fprintf(b, "Content-Length: %d\r\n", len(body))
	b.WriteString("\r\n")
	b.Write(body)

	return []byte(b.String()), nil
}

func ensureToTag(to string) string {
	// If it already has a tag, leave it alone.
	if strings.Contains(strings.ToLower(to), "tag=") {
		return to
	}
	// Append a simple tag token.
	tag := fmt.Sprintf("%d", time.Now().UnixNano())
	sep := ";"
	if strings.Contains(to, ";") {
		sep = ""
	}
	return to + sep + "tag=" + tag
}

func defaultReason(code int) string {
	switch code {
	case 200:
		return "OK"
	case 100:
		return "Trying"
	case 180:
		return "Ringing"
	case 183:
		return "Session Progress"
	case 400:
		return "Bad Request"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 408:
		return "Request Timeout"
	case 302:
		return "Moved Temporarily"
	case 481:
		return "Call/Transaction Does Not Exist"
	case 486:
		return "Busy Here"
	case 500:
		return "Server Internal Error"
	case 501:
		return "Not Implemented"
	case 503:
		return "Service Unavailable"
	default:
		return "" // caller can provide
	}
}
