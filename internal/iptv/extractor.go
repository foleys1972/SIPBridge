package iptv

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type Extractor struct {
	conn   *net.UDPConn
	cmd    *exec.Cmd
	stopCh chan struct{}
	once   sync.Once
}

type jitterBuffer struct {
	delay time.Duration
	in    chan []byte
	stop  chan struct{}
}

func newJitterBuffer(delayMs int, onPacket func([]byte)) *jitterBuffer {
	jb := &jitterBuffer{
		delay: time.Duration(delayMs) * time.Millisecond,
		in:    make(chan []byte, 512),
		stop:  make(chan struct{}),
	}
	go func() {
		if jb.delay <= 0 {
			for {
				select {
				case <-jb.stop:
					return
				case pkt := <-jb.in:
					onPacket(pkt)
				}
			}
		}
		for {
			select {
			case <-jb.stop:
				return
			case pkt := <-jb.in:
				time.Sleep(jb.delay)
				onPacket(pkt)
			}
		}
	}()
	return jb
}

func (jb *jitterBuffer) Push(pkt []byte) {
	if jb == nil {
		return
	}
	select {
	case jb.in <- pkt:
	default:
	}
}

func (jb *jitterBuffer) Close() {
	if jb == nil {
		return
	}
	close(jb.stop)
}

func StartFFmpegAudioExtract(multicastIP string, port int, jitterMs int, onPacket func([]byte), loggerPrefix string) (*Extractor, error) {
	if parseMulticastIPv4(multicastIP) == nil || port <= 0 || port > 65535 {
		return nil, net.InvalidAddrError("invalid multicast ip/port")
	}
	if onPacket == nil {
		return nil, fmt.Errorf("onPacket is required")
	}
	udpIn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		return nil, err
	}
	localPort := udpIn.LocalAddr().(*net.UDPAddr).Port
	inputURL := fmt.Sprintf("udp://@%s:%d?fifo_size=5000000&overrun_nonfatal=1", multicastIP, port)
	outputURL := fmt.Sprintf("rtp://127.0.0.1:%d", localPort)

	ffmpegBin, err := resolveFFmpegBinary()
	if err != nil {
		_ = udpIn.Close()
		return nil, err
	}
	cmd := exec.Command(
		ffmpegBin,
		"-hide_banner", "-loglevel", "warning",
		"-i", inputURL,
		"-vn",
		"-acodec", "pcm_mulaw",
		"-ar", "8000",
		"-ac", "1",
		"-f", "rtp",
		outputURL,
	)
	if err := cmd.Start(); err != nil {
		_ = udpIn.Close()
		return nil, err
	}

	ex := &Extractor{
		conn:   udpIn,
		cmd:    cmd,
		stopCh: make(chan struct{}),
	}
	jb := newJitterBuffer(jitterMs, onPacket)

	go func() {
		buf := make([]byte, 4096)
		for {
			n, _, err := udpIn.ReadFromUDP(buf)
			if err != nil {
				select {
				case <-ex.stopCh:
					return
				default:
				}
				log.Printf("%s read err=%v", loggerPrefix, err)
				continue
			}
			if n < 12 {
				continue
			}
			pkt := append([]byte(nil), buf[:n]...)
			jb.Push(pkt)
		}
	}()
	go func() {
		_ = cmd.Wait()
	}()
	go func() {
		<-ex.stopCh
		jb.Close()
	}()
	return ex, nil
}

func resolveFFmpegBinary() (string, error) {
	if p := filepath.Clean(filepath.FromSlash(os.Getenv("SIPBRIDGE_FFMPEG_PATH"))); p != "." && p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}
	candidates := []string{
		filepath.Join(".", "ffmpeg.exe"),
		filepath.Join(".", "ffmpeg"),
		filepath.Join(".", "tools", "ffmpeg", "ffmpeg.exe"),
		filepath.Join(".", "tools", "ffmpeg", "ffmpeg"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("ffmpeg not found: set SIPBRIDGE_FFMPEG_PATH or place ffmpeg in PATH/project folder")
}

type FFmpegDiagnostic struct {
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
	Found bool   `json:"found"`
}

func FFmpegBinaryDiagnostic() FFmpegDiagnostic {
	p, err := resolveFFmpegBinary()
	if err != nil {
		return FFmpegDiagnostic{Found: false, Error: err.Error()}
	}
	return FFmpegDiagnostic{Found: true, Path: p}
}

func (e *Extractor) Close() {
	if e == nil {
		return
	}
	e.once.Do(func() {
		close(e.stopCh)
		if e.cmd != nil && e.cmd.Process != nil {
			_ = e.cmd.Process.Kill()
		}
		if e.conn != nil {
			_ = e.conn.Close()
		}
	})
}
