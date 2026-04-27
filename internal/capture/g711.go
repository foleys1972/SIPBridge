package capture

// G.711 µ-law and A-law decode to 16-bit linear PCM.
// Algorithms: ITU-T G.711 / Sun Microsystems g711.c reference implementation.

const g711Bias = 0x84 // µ-law expansion bias

// MuLawDecode converts a single G.711 µ-law encoded byte to 16-bit signed
// linear PCM (8 kHz sample rate, as used in SIP/RTP PCMU payload type 0).
func MuLawDecode(u byte) int16 {
	u = ^u
	t := int(u&0x0F)<<3 + g711Bias
	t <<= (u & 0x70) >> 4
	if u&0x80 != 0 {
		return int16(g711Bias - t)
	}
	return int16(t - g711Bias)
}

// ALawDecode converts a single G.711 A-law encoded byte to 16-bit signed
// linear PCM (8 kHz sample rate, as used in SIP/RTP PCMA payload type 8).
func ALawDecode(a byte) int16 {
	a ^= 0x55
	seg := (a & 0x70) >> 4
	q := int(a&0x0F) << 1
	var t int
	switch seg {
	case 0:
		t = q + 1
	case 1:
		t = q + 0x21
	default:
		t = (q + 0x21) << (seg - 1)
	}
	if a&0x80 != 0 {
		return int16(t)
	}
	return int16(-t)
}

// DecodePCMU decodes a PCMU (G.711 µ-law) RTP payload to 16-bit LE PCM
// samples. Output length equals input length.
func DecodePCMU(payload []byte) []int16 {
	out := make([]int16, len(payload))
	for i, b := range payload {
		out[i] = MuLawDecode(b)
	}
	return out
}

// DecodePCMA decodes a PCMA (G.711 A-law) RTP payload to 16-bit LE PCM
// samples. Output length equals input length.
func DecodePCMA(payload []byte) []int16 {
	out := make([]int16, len(payload))
	for i, b := range payload {
		out[i] = ALawDecode(b)
	}
	return out
}
