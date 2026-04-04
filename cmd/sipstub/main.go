package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"sipbridge/internal/sip"
)

type mode string

const (
	modeWinner  mode = "winner"
	modeTimeout mode = "timeout"
	modeBusy    mode = "busy"
)

func main() {
	bindAddr := envOr("SIPSTUB_BIND_ADDR", "127.0.0.1")
	port := envOrInt("SIPSTUB_PORT", 5070)
	m := mode(strings.ToLower(strings.TrimSpace(envOr("SIPSTUB_MODE", string(modeWinner)))))
	answerDelay := time.Duration(envOrInt("SIPSTUB_ANSWER_DELAY_MS", 2000)) * time.Millisecond

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	addr := &net.UDPAddr{IP: net.ParseIP(bindAddr), Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("listen udp: %v", err)
	}
	defer conn.Close()

	log.Printf("sipstub listening on %s mode=%s answerDelay=%s", conn.LocalAddr().String(), m, answerDelay)

	var mu sync.Mutex
	inviteOrder := 0
	seenInvite := make(map[string]int)

	buf := make([]byte, 64*1024)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remote, err := conn.ReadFromUDP(buf)
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err != nil {
			nErr, ok := err.(net.Error)
			if ok && nErr.Timeout() {
				continue
			}
			log.Printf("read udp: %v", err)
			continue
		}

		payload := append([]byte(nil), buf[:n]...)
		msg, pErr := sip.ParseMessage(payload)
		if pErr != nil {
			log.Printf("parse error from=%s err=%v", remote.String(), pErr)
			continue
		}

		if msg.IsRequest {
			switch msg.Method {
			case "INVITE":
				callID := strings.TrimSpace(msg.Header("call-id"))
				mu.Lock()
				idx, ok := seenInvite[callID]
				if !ok {
					idx = inviteOrder
					inviteOrder++
					seenInvite[callID] = idx
				}
				mu.Unlock()

				ringing, _ := sip.BuildResponse(msg, 180, "Ringing", nil, nil)
				_, _ = conn.WriteToUDP(ringing, remote)

				switch m {
				case modeTimeout:
					continue
				case modeBusy:
					go func() {
						time.Sleep(200 * time.Millisecond)
						busy, _ := sip.BuildResponse(msg, 486, "Busy Here", nil, nil)
						_, _ = conn.WriteToUDP(busy, remote)
					}()
					continue
				default:
				}

				if idx == 0 {
					go func() {
						time.Sleep(answerDelay)
						sdp := buildMinimalSDP("127.0.0.1", 40000)
						extra := map[string]string{
							"Content-Type": "application/sdp",
							"Contact":      "<sip:sipstub@127.0.0.1>",
							"Allow":        "INVITE, ACK, BYE, CANCEL, OPTIONS",
						}
						okResp, _ := sip.BuildResponse(msg, 200, "OK", extra, []byte(sdp))
						_, _ = conn.WriteToUDP(okResp, remote)
					}()
				} else {
					go func() {
						time.Sleep(300 * time.Millisecond)
						busy, _ := sip.BuildResponse(msg, 486, "Busy Here", nil, nil)
						_, _ = conn.WriteToUDP(busy, remote)
					}()
				}
			case "CANCEL":
				okResp, _ := sip.BuildResponse(msg, 200, "OK", nil, nil)
				_, _ = conn.WriteToUDP(okResp, remote)
			case "BYE":
				okResp, _ := sip.BuildResponse(msg, 200, "OK", nil, nil)
				_, _ = conn.WriteToUDP(okResp, remote)
			default:
				resp, _ := sip.BuildResponse(msg, 405, "Method Not Allowed", map[string]string{"Allow": "INVITE, CANCEL, BYE"}, nil)
				_, _ = conn.WriteToUDP(resp, remote)
			}
		}
	}
}

func buildMinimalSDP(ip string, port int) string {
	return fmt.Sprintf("v=0\r\no=sipstub 0 0 IN IP4 %s\r\ns=sipstub\r\nc=IN IP4 %s\r\nt=0 0\r\nm=audio %d RTP/AVP 0 8\r\na=rtpmap:0 PCMU/8000\r\na=rtpmap:8 PCMA/8000\r\na=inactive\r\n", ip, ip, port)
}

func envOr(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func envOrInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
