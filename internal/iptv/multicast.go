package iptv

import (
	"log"
	"net"
	"strings"
	"sync"
)

type Stream struct {
	conn   *net.UDPConn
	stopCh chan struct{}
	once   sync.Once
}

func parseMulticastIPv4(ip string) net.IP {
	addr := net.ParseIP(strings.TrimSpace(ip))
	if addr == nil {
		return nil
	}
	v4 := addr.To4()
	if v4 == nil || !v4.IsMulticast() {
		return nil
	}
	return v4
}

func StartMulticast(multicastIP string, port int, onPacket func([]byte), loggerPrefix string) (*Stream, error) {
	ip := parseMulticastIPv4(multicastIP)
	if ip == nil || port <= 0 || port > 65535 {
		return nil, net.InvalidAddrError("invalid multicast ip/port")
	}
	group := &net.UDPAddr{IP: ip, Port: port}
	conn, err := net.ListenMulticastUDP("udp4", nil, group)
	if err != nil {
		return nil, err
	}
	_ = conn.SetReadBuffer(1 << 20)
	s := &Stream{
		conn:   conn,
		stopCh: make(chan struct{}),
	}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				select {
				case <-s.stopCh:
					return
				default:
				}
				log.Printf("%s read err=%v", loggerPrefix, err)
				continue
			}
			if n < 12 {
				continue
			}
			if onPacket != nil {
				pkt := append([]byte(nil), buf[:n]...)
				onPacket(pkt)
			}
		}
	}()
	return s, nil
}

func (s *Stream) Close() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		close(s.stopCh)
		if s.conn != nil {
			_ = s.conn.Close()
		}
	})
}
