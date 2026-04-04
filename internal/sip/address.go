package sip

import "strings"

// ExtractURIFromAddressHeader extracts the SIP/SIPS URI portion from a header like:
//   From: "Alice" <sip:1001@example.com>;tag=abc
//   To: <sip:1000@example.com>
//   From: sip:1001@example.com;tag=abc
func ExtractURIFromAddressHeader(h string) string {
	s := strings.TrimSpace(h)
	if s == "" {
		return ""
	}
	// Prefer angle-bracket form
	if lt := strings.Index(s, "<"); lt >= 0 {
		if gt := strings.Index(s[lt+1:], ">" ); gt >= 0 {
			return strings.TrimSpace(s[lt+1 : lt+1+gt])
		}
	}
	// Fallback: take up to first ';'
	if semi := strings.Index(s, ";"); semi >= 0 {
		s = s[:semi]
	}
	// Remove any surrounding quotes
	s = strings.Trim(s, "\"")
	// If there's whitespace (display-name), take last token
	fields := strings.Fields(s)
	if len(fields) > 0 {
		s = fields[len(fields)-1]
	}
	return strings.TrimSpace(s)
}
