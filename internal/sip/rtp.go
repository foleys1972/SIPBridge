package sip

import (
	"encoding/binary"
	"log"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"
)

type rtpSession struct {
	mu sync.Mutex

	conn   *net.UDPConn
	remote *net.UDPAddr

	ssrc      uint32
	seq       uint16
	timestamp uint32
	pt        uint8

	telephoneEventPT uint8
	onDTMFDigit      func(d string)
	onRTPPacket      func(pkt []byte)
	lastEvent        uint8
	lastEventAt      time.Time

	loggedFirstRx bool

	silenceStarted bool

	closed bool
}

func newRTPSession(conn *net.UDPConn, remote *net.UDPAddr, payloadType uint8, telephoneEventPT uint8, onDTMFDigit func(d string)) *rtpSession {
	return &rtpSession{
		conn:      conn,
		remote:    remote,
		ssrc:      rand.Uint32(),
		seq:       uint16(rand.Intn(65535)),
		timestamp: rand.Uint32(),
		pt:        payloadType,
		telephoneEventPT: telephoneEventPT,
		onDTMFDigit:      onDTMFDigit,
		lastEvent:        255,
	}
}

func (s *rtpSession) SetOnRTPPacket(cb func(pkt []byte)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.onRTPPacket = cb
	s.mu.Unlock()
}

func (s *rtpSession) LocalPort() int {
	if s == nil || s.conn == nil {
		return 0
	}
	if a, ok := s.conn.LocalAddr().(*net.UDPAddr); ok && a != nil {
		return a.Port
	}
	return 0
}

func (s *rtpSession) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *rtpSession) StartSilence() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed || s.silenceStarted {
		s.mu.Unlock()
		return
	}
	s.silenceStarted = true
	s.mu.Unlock()

	go func() {
		const sampleRate = 8000
		const frameMs = 20
		const samplesPerFrame = sampleRate * frameMs / 1000 // 160
		sil := make([]byte, samplesPerFrame)
		for i := range sil {
			// mu-law silence is typically 0xFF
			sil[i] = 0xFF
		}

		t := time.NewTicker(frameMs * time.Millisecond)
		defer t.Stop()
		for {
			<-t.C
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				return
			}
			s.timestamp += samplesPerFrame
			s.mu.Unlock()
			s.send(sil, false)
		}
	}()
}

func (s *rtpSession) StartReceiver() {
	if s == nil {
		return
	}
	if s.conn == nil {
		return
	}

	go func() {
		buf := make([]byte, 2048)
		for {
			_ = s.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, _, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				s.mu.Lock()
				closed := s.closed
				s.mu.Unlock()
				if closed {
					return
				}
				nErr, ok := err.(net.Error)
				if ok && nErr.Timeout() {
					continue
				}
				continue
			}
			if n < 12 {
				continue
			}

			s.mu.Lock()
			if !s.loggedFirstRx {
				s.loggedFirstRx = true
				s.mu.Unlock()
				log.Printf("RTP rx first_packet bytes=%d", n)
			} else {
				s.mu.Unlock()
			}

			v := (buf[0] >> 6) & 0x03
			if v != 2 {
				continue
			}
			cc := int(buf[0] & 0x0f)
			pt := buf[1] & 0x7f
			hdrLen := 12 + 4*cc
			if n < hdrLen+4 {
				continue
			}
			// If this is a telephone-event packet, decode it.
			s.mu.Lock()
			telPT := s.telephoneEventPT
			s.mu.Unlock()
			if telPT != 0 && pt == telPT {
				log.Printf("RTP rx telephone-event packet pt=%d bytes=%d", pt, n)
				payload := buf[hdrLen:n]
				s.decodeTelephoneEvent(payload)
				continue
			}

			// Forward non-DTMF RTP (audio) if configured.
			s.mu.Lock()
			cb := s.onRTPPacket
			s.mu.Unlock()
			if cb != nil {
				pktCopy := append([]byte(nil), buf[:n]...)
				cb(pktCopy)
			}
		}
	}()
}

func (s *rtpSession) decodeTelephoneEvent(payload []byte) {
	if s == nil || len(payload) < 4 {
		return
	}
	event := payload[0]
	end := (payload[1] & 0x80) != 0
	if !end {
		return
	}

	cb := func() func(d string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		// Many endpoints repeat the RTP event multiple times with end=1. Only drop
		// duplicates that arrive extremely close together.
		now := time.Now()
		if s.lastEvent == event && !s.lastEventAt.IsZero() && now.Sub(s.lastEventAt) < 150*time.Millisecond {
			return nil
		}
		s.lastEvent = event
		s.lastEventAt = now
		return s.onDTMFDigit
	}()
	if cb == nil {
		return
	}

	d := ""
	switch {
	case event <= 9:
		d = string('0' + event)
	case event == 10:
		d = "*"
	case event == 11:
		d = "#"
	case event == 12:
		d = "A"
	case event == 13:
		d = "B"
	case event == 14:
		d = "C"
	case event == 15:
		d = "D"
	}
	if d != "" {
		log.Printf("RTP DTMF decoded digit=%s event=%d", d, event)
		cb(d)
	}
}

func (s *rtpSession) send(payload []byte, marker bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.conn == nil || s.remote == nil {
		return
	}

	hdr := make([]byte, 12)
	hdr[0] = 0x80
	if marker {
		hdr[1] = 0x80 | (s.pt & 0x7f)
	} else {
		hdr[1] = s.pt & 0x7f
	}
	binary.BigEndian.PutUint16(hdr[2:4], s.seq)
	binary.BigEndian.PutUint32(hdr[4:8], s.timestamp)
	binary.BigEndian.PutUint32(hdr[8:12], s.ssrc)

	pkt := append(hdr, payload...)
	_, _ = s.conn.WriteToUDP(pkt, s.remote)

	s.seq++
}

// PlayTone sends a short tone to the remote using PCMU.
// freqHz is the tone frequency; dur is total duration; amp is [0..1].
func (s *rtpSession) PlayTone(freqHz float64, dur time.Duration, amp float64) {
	if s == nil {
		return
	}
	if amp < 0 {
		amp = 0
	}
	if amp > 1 {
		amp = 1
	}

	const sampleRate = 8000
	const frameMs = 20
	const samplesPerFrame = sampleRate * frameMs / 1000 // 160

	totalSamples := int(float64(sampleRate) * dur.Seconds())
	sent := 0

	ticker := time.NewTicker(frameMs * time.Millisecond)
	defer ticker.Stop()

	phase := 0.0
	phaseInc := 2 * math.Pi * freqHz / sampleRate

	for sent < totalSamples {
		<-ticker.C
		buf := make([]byte, samplesPerFrame)
		for i := 0; i < samplesPerFrame; i++ {
			sample := math.Sin(phase) * amp
			phase += phaseInc
			if phase > 2*math.Pi {
				phase -= 2 * math.Pi
			}
			// sample in [-1,1] -> int16 PCM
			pcm := int16(sample * 30000)
			buf[i] = linearToMuLaw(pcm)
		}
		s.send(buf, sent == 0)
		// advance RTP timestamp by number of samples sent
		s.mu.Lock()
		s.timestamp += samplesPerFrame
		s.mu.Unlock()
		sent += samplesPerFrame
	}
}

func linearToMuLaw(sample int16) byte {
	// G.711 µ-law encoder (8-bit). Adapted to be dependency-free.
	const (
		bias = 0x84
		clip = 32635
	)

	s := int(sample)
	sign := 0
	if s < 0 {
		sign = 0x80
		s = -s
	}
	if s > clip {
		s = clip
	}
	s += bias

	// Determine exponent.
	exp := 7
	mask := 0x4000
	for exp > 0 && (s&mask) == 0 {
		exp--
		mask >>= 1
	}

	mantissa := (s >> (exp + 3)) & 0x0F
	ulaw := ^byte(sign | (exp << 4) | mantissa)
	return ulaw
}
