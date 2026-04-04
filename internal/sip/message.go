package sip

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Message struct {
	IsRequest bool

	Method  string
	RequestURI string

	StatusCode int
	Reason     string

	Version string
	Headers map[string][]string
	Body    []byte
	Raw     []byte
}

func ParseMessage(b []byte) (*Message, error) {
	m := &Message{Headers: make(map[string][]string), Raw: append([]byte(nil), b...)}

	r := bufio.NewReader(bytes.NewReader(b))
	startLine, err := readLine(r)
	if err != nil {
		return nil, fmt.Errorf("read start-line: %w", err)
	}
	if startLine == "" {
		return nil, fmt.Errorf("empty start-line")
	}

	if strings.HasPrefix(startLine, "SIP/") {
		m.IsRequest = false
		parts := strings.SplitN(startLine, " ", 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("bad status line: %q", startLine)
		}
		m.Version = parts[0]
		code, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("bad status code: %q", parts[1])
		}
		m.StatusCode = code
		m.Reason = strings.TrimSpace(parts[2])
	} else {
		m.IsRequest = true
		parts := strings.SplitN(startLine, " ", 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("bad request line: %q", startLine)
		}
		m.Method = strings.TrimSpace(parts[0])
		m.RequestURI = strings.TrimSpace(parts[1])
		m.Version = strings.TrimSpace(parts[2])
	}

	for {
		line, err := readLine(r)
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key := canonicalHeaderKey(strings.TrimSpace(k))
		val := strings.TrimSpace(v)
		m.Headers[key] = append(m.Headers[key], val)
	}

	cl := 0
	if v := m.Header("content-length"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			cl = n
		}
	}
	m.Body = make([]byte, cl)
	if cl > 0 {
		if _, err := io.ReadFull(r, m.Body); err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}
	}

	return m, nil
}

func (m *Message) Header(key string) string {
	vals := m.Headers[canonicalHeaderKey(key)]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (m *Message) AllHeaders(key string) []string {
	return append([]string(nil), m.Headers[canonicalHeaderKey(key)]...)
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		// It might be the last line without \n
		if len(line) == 0 {
			return "", err
		}
	}
	line = strings.TrimRight(line, "\r\n")
	return line, nil
}

func canonicalHeaderKey(k string) string {
	k = strings.TrimSpace(k)
	k = strings.ToLower(k)
	switch k {
	case "f":
		return "from"
	case "t":
		return "to"
	case "v":
		return "via"
	case "i":
		return "call-id"
	case "l":
		return "content-length"
	case "c":
		return "content-type"
	case "m":
		return "contact"
	}
	return k
}
