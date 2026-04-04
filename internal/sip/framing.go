package sip

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ReadNextSIPMessage reads one complete SIP message from a TCP/TLS stream using Content-Length framing (RFC 3261).
func ReadNextSIPMessage(br *bufio.Reader) ([]byte, error) {
	var hdr bytes.Buffer
	first, err := readLineBytes(br)
	if err != nil {
		return nil, err
	}
	if len(first) == 0 {
		return nil, fmt.Errorf("empty start-line")
	}
	hdr.Write(first)
	hdr.WriteString("\r\n")
	for {
		line, err := readLineBytes(br)
		if err != nil {
			return nil, err
		}
		hdr.Write(line)
		hdr.WriteString("\r\n")
		if len(line) == 0 {
			break
		}
	}
	cl := contentLengthFromHeaderBlock(hdr.Bytes())
	if cl < 0 {
		cl = 0
	}
	body := make([]byte, cl)
	if cl > 0 {
		if _, err := io.ReadFull(br, body); err != nil {
			return nil, err
		}
	}
	out := append(hdr.Bytes(), body...)
	return out, nil
}

func readLineBytes(br *bufio.Reader) ([]byte, error) {
	line, err := br.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return nil, err
	}
	for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r') {
		line = line[:len(line)-1]
	}
	return line, err
}

func contentLengthFromHeaderBlock(header []byte) int {
	s := string(header)
	lines := strings.Split(s, "\r\n")
	for _, line := range lines {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(k), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return -1
			}
			return n
		}
	}
	return 0
}
