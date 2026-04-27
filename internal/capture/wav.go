package capture

import (
	"encoding/binary"
	"fmt"
	"os"
)

// wavWriter streams 16-bit mono PCM audio to a WAV file.
// The RIFF/data chunk sizes are written as placeholders and patched on Close.
//
// Format: PCM, 1 channel, 8000 Hz, 16-bit LE (format tag 1).
const (
	wavSampleRate  = 8000
	wavChannels    = 1
	wavBitsPerSamp = 16
	wavByteRate    = wavSampleRate * wavChannels * wavBitsPerSamp / 8
	wavBlockAlign  = wavChannels * wavBitsPerSamp / 8
	wavHeaderSize  = 44
)

type wavWriter struct {
	f         *os.File
	bytesData uint32
}

func newWAVWriter(path string) (*wavWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create wav %s: %w", path, err)
	}
	w := &wavWriter{f: f}
	if err := w.writeHeader(0); err != nil {
		_ = f.Close()
		return nil, err
	}
	return w, nil
}

func (w *wavWriter) writeHeader(dataBytes uint32) error {
	riffSize := 36 + dataBytes
	hdr := make([]byte, wavHeaderSize)
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], riffSize)
	copy(hdr[8:12], "WAVE")
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)             // fmt chunk size
	binary.LittleEndian.PutUint16(hdr[20:22], 1)              // PCM
	binary.LittleEndian.PutUint16(hdr[22:24], wavChannels)
	binary.LittleEndian.PutUint32(hdr[24:28], wavSampleRate)
	binary.LittleEndian.PutUint32(hdr[28:32], wavByteRate)
	binary.LittleEndian.PutUint16(hdr[32:34], wavBlockAlign)
	binary.LittleEndian.PutUint16(hdr[34:36], wavBitsPerSamp)
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], dataBytes)
	_, err := w.f.WriteAt(hdr, 0)
	return err
}

// WritePCMU decodes a PCMU (G.711 µ-law) payload and appends to the WAV file.
func (w *wavWriter) WritePCMU(payload []byte) error {
	return w.writeSamples(DecodePCMU(payload))
}

// WritePCMA decodes a PCMA (G.711 A-law) payload and appends to the WAV file.
func (w *wavWriter) WritePCMA(payload []byte) error {
	return w.writeSamples(DecodePCMA(payload))
}

func (w *wavWriter) writeSamples(samples []int16) error {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	n, err := w.f.Write(buf)
	w.bytesData += uint32(n)
	return err
}

// BytesWritten returns the number of PCM data bytes written so far.
func (w *wavWriter) BytesWritten() uint32 { return w.bytesData }

// DurationSeconds returns the approximate audio duration based on bytes written.
func (w *wavWriter) DurationSeconds() float64 {
	return float64(w.bytesData) / float64(wavByteRate)
}

// Close patches the WAV header with the final sizes and closes the file.
func (w *wavWriter) Close() error {
	if err := w.writeHeader(w.bytesData); err != nil {
		_ = w.f.Close()
		return err
	}
	return w.f.Close()
}
