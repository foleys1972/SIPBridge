package sip

import "strings"

// MaskPINDigits returns a bullet string for UI display; it does not reveal the secret.
func MaskPINDigits(length int) string {
	if length <= 0 {
		return ""
	}
	n := length
	if n > 12 {
		n = 12
	}
	return strings.Repeat("•", n)
}
