package sip

import (
	"fmt"
	"net"
)

// sipBackchannel is how SIPBridge reaches the original inbound caller (UDP or TLS/TCP).
type sipBackchannel struct {
	UDP  *net.UDPConn
	Peer *net.UDPAddr
	Conn net.Conn
}

func (bc sipBackchannel) Write(msg []byte) error {
	if bc.Conn != nil {
		return writeFull(bc.Conn, msg)
	}
	if bc.UDP != nil && bc.Peer != nil {
		_, err := bc.UDP.WriteToUDP(msg, bc.Peer)
		return err
	}
	return fmt.Errorf("sip backchannel: no transport")
}

func (bc sipBackchannel) LocalAddr() net.Addr {
	if bc.Conn != nil {
		return bc.Conn.LocalAddr()
	}
	if bc.UDP != nil {
		return bc.UDP.LocalAddr()
	}
	return nil
}

func writeFull(c net.Conn, b []byte) error {
	for len(b) > 0 {
		n, err := c.Write(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}
