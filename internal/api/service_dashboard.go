package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"sipbridge/internal/sip"
)

const dashboardLogMax = 80

type dashboardLogEntry struct {
	TS      string `json:"ts"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type serviceRow struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
}

type recorderDashboardSnapshot struct {
	OK        bool            `json:"ok"`
	URL       string          `json:"url,omitempty"`
	Error     string          `json:"error,omitempty"`
	LatencyMs int64           `json:"latency_ms,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type serviceDashboardResponse struct {
	CheckedAt    string                       `json:"checked_at"`
	SummaryUp    int                          `json:"summary_up"`
	SummaryTotal int                          `json:"summary_total"`
	Services     []serviceRow                 `json:"services"`
	Log          []dashboardLogEntry          `json:"log"`
	Note         string                       `json:"note"`
	Recorder     *recorderDashboardSnapshot   `json:"recorder,omitempty"`
}

var (
	dashboardLogMu sync.Mutex
	dashboardLog   []dashboardLogEntry
)

func appendDashboardLog(level, msg string) {
	dashboardLogMu.Lock()
	defer dashboardLogMu.Unlock()
	dashboardLog = append(dashboardLog, dashboardLogEntry{
		TS:      time.Now().UTC().Format(time.RFC3339),
		Level:   level,
		Message: msg,
	})
	if len(dashboardLog) > dashboardLogMax {
		dashboardLog = dashboardLog[len(dashboardLog)-dashboardLogMax:]
	}
}

func snapshotDashboardLog() []dashboardLogEntry {
	dashboardLogMu.Lock()
	defer dashboardLogMu.Unlock()
	out := make([]dashboardLogEntry, len(dashboardLog))
	copy(out, dashboardLog)
	return out
}

func envOrDefault(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func probeTCP(ctx context.Context, host string, port int, label, id string) serviceRow {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	t0 := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	lat := time.Since(t0).Milliseconds()
	if err != nil {
		return serviceRow{ID: id, Label: label, Status: "down", Detail: err.Error(), LatencyMs: lat}
	}
	_ = conn.Close()
	return serviceRow{ID: id, Label: label, Status: "up", Detail: "tcp ok", LatencyMs: lat}
}

func probeUDPWrite(ctx context.Context, host string, port int, label, id string) serviceRow {
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return serviceRow{ID: id, Label: label, Status: "down", Detail: err.Error()}
	}
	t0 := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", addr.String())
	lat := time.Since(t0).Milliseconds()
	if err != nil {
		return serviceRow{ID: id, Label: label, Status: "down", Detail: err.Error(), LatencyMs: lat}
	}
	defer conn.Close()
	_, werr := conn.Write([]byte{0x00})
	if werr != nil {
		return serviceRow{ID: id, Label: label, Status: "down", Detail: werr.Error(), LatencyMs: lat}
	}
	return serviceRow{ID: id, Label: label, Status: "up", Detail: "udp send ok", LatencyMs: lat}
}

func probeHTTPHealth(ctx context.Context, url, label, id string) serviceRow {
	t0 := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return serviceRow{ID: id, Label: label, Status: "down", Detail: err.Error()}
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	lat := time.Since(t0).Milliseconds()
	if err != nil {
		return serviceRow{ID: id, Label: label, Status: "down", Detail: err.Error(), LatencyMs: lat}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return serviceRow{ID: id, Label: label, Status: "up", Detail: fmt.Sprintf("HTTP %d", resp.StatusCode), LatencyMs: lat}
	}
	return serviceRow{ID: id, Label: label, Status: "degraded", Detail: fmt.Sprintf("HTTP %d", resp.StatusCode), LatencyMs: lat}
}

func probeSIPRECPath(ctx context.Context, sipSrv *sip.Server) serviceRow {
	if sipSrv == nil {
		return serviceRow{ID: "siprec_options", Label: "SIPREC path (OPTIONS to recorder)", Status: "unknown", Detail: "sip server not available"}
	}
	t0 := time.Now()
	r := sipSrv.ProbeSIPREC(ctx)
	lat := time.Since(t0).Milliseconds()
	switch {
	case r.OK && r.Reachable:
		return serviceRow{ID: "siprec_options", Label: "SIPREC path (OPTIONS to recorder)", Status: "up", Detail: fmt.Sprintf("OPTIONS %d", r.SIPStatus), LatencyMs: lat}
	case r.Reachable && r.SIPStatus == 503:
		return serviceRow{ID: "siprec_options", Label: "SIPREC path (OPTIONS to recorder)", Status: "degraded", Detail: "503 — no app on drachtio (Node not running or secret mismatch vs drachtio)", LatencyMs: lat}
	case r.Reachable:
		return serviceRow{ID: "siprec_options", Label: "SIPREC path (OPTIONS to recorder)", Status: "degraded", Detail: r.Error, LatencyMs: lat}
	default:
		d := r.Error
		if d == "" {
			d = r.Step
		}
		return serviceRow{ID: "siprec_options", Label: "SIPREC path (OPTIONS to recorder)", Status: "down", Detail: d, LatencyMs: lat}
	}
}

func fetchRecorderDashboard(ctx context.Context) *recorderDashboardSnapshot {
	base := strings.TrimSpace(envOrDefault("SIPREC_RECORDER_BASE_URL", "http://127.0.0.1:3030"))
	if strings.EqualFold(base, "none") || base == "-" {
		return nil
	}
	base = strings.TrimRight(base, "/")
	u := base + "/api/health/dashboard"
	t0 := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return &recorderDashboardSnapshot{OK: false, URL: u, Error: err.Error()}
	}
	client := &http.Client{Timeout: 14 * time.Second}
	resp, err := client.Do(req)
	lat := time.Since(t0).Milliseconds()
	if err != nil {
		return &recorderDashboardSnapshot{OK: false, URL: u, Error: err.Error(), LatencyMs: lat}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &recorderDashboardSnapshot{OK: false, URL: u, Error: err.Error(), LatencyMs: lat}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &recorderDashboardSnapshot{OK: false, URL: u, Error: fmt.Sprintf("HTTP %d", resp.StatusCode), LatencyMs: lat}
	}
	trim := bytes.TrimSpace(body)
	if !json.Valid(trim) {
		return &recorderDashboardSnapshot{OK: false, URL: u, Error: "invalid JSON from recorder", LatencyMs: lat}
	}
	return &recorderDashboardSnapshot{OK: true, URL: u, LatencyMs: lat, Payload: json.RawMessage(trim)}
}

func (s *Server) runServiceDashboard(ctx context.Context) serviceDashboardResponse {
	host := envOrDefault("SIPBRIDGE_STACK_HOST", "127.0.0.1")
	siprecURL := envOrDefault("SIPREC_HEALTH_URL", "http://127.0.0.1:3030/api/health")

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Determine whether SIPREC recorder stack checks should be included.
	siprecEnabled := false
	if s.cm != nil {
		rec := s.cm.Current().Spec.Recording
		siprecEnabled = rec != nil && rec.GlobalEnabled && rec.SIPREC != nil && rec.SIPREC.Enabled
	}

	recCh := make(chan *recorderDashboardSnapshot, 1)
	go func() {
		if siprecEnabled {
			recCh <- fetchRecorderDashboard(ctx)
		} else {
			recCh <- nil
		}
	}()

	type job struct {
		fn func() serviceRow
	}
	jobs := []job{
		{fn: func() serviceRow {
			return serviceRow{ID: "sipbridge_api", Label: "SIP Bridge API (this process)", Status: "up", Detail: "handler running"}
		}},
	}

	if siprecEnabled {
		jobs = append(jobs,
			job{fn: func() serviceRow {
				r := probeTCP(ctx, host, 9022, "Drachtio control (TCP 9022)", "drachtio_control")
				if r.Status == "up" {
					r.Detail = "tcp ok (listener only; not Node-to-drachtio app registration)"
				}
				return r
			}},
			job{fn: func() serviceRow { return probeTCP(ctx, host, 5065, "Drachtio SIP (TCP 5065)", "drachtio_sip_tcp") }},
			job{fn: func() serviceRow { return probeUDPWrite(ctx, host, 5065, "Drachtio SIP (UDP 5065)", "drachtio_sip_udp") }},
			job{fn: func() serviceRow { return probeUDPWrite(ctx, host, 22222, "rtpengine ng (UDP 22222)", "rtpengine_ng") }},
			job{fn: func() serviceRow { return probeTCP(ctx, host, 5433, "PostgreSQL (TCP 5433)", "postgres") }},
			job{fn: func() serviceRow { return probeHTTPHealth(ctx, siprecURL, "SIPREC admin API (HTTP)", "siprec_admin") }},
			job{fn: func() serviceRow { return probeSIPRECPath(ctx, s.sipSrv) }},
		)
	}

	rows := make([]serviceRow, len(jobs))
	var wg sync.WaitGroup
	for i := range jobs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rows[idx] = jobs[idx].fn()
		}(i)
	}
	wg.Wait()

	order := []string{
		"sipbridge_api", "drachtio_control", "drachtio_sip_tcp", "drachtio_sip_udp",
		"rtpengine_ng", "postgres", "siprec_admin", "siprec_options",
	}
	byID := make(map[string]serviceRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	ordered := make([]serviceRow, 0, len(order))
	for _, id := range order {
		if r, ok := byID[id]; ok {
			ordered = append(ordered, r)
		}
	}

	up := 0
	for _, r := range ordered {
		if r.Status == "up" {
			up++
		}
	}
	appendDashboardLog("info", fmt.Sprintf("%d/%d checks up (green)", up, len(ordered)))

	rec := <-recCh
	if rec != nil && rec.OK {
		appendDashboardLog("info", "Merged SIPREC recorder dashboard")
	} else if rec != nil && !rec.OK {
		appendDashboardLog("warn", "SIPREC recorder dashboard: "+rec.Error)
	}

	note := "SIP Bridge: SIPBRIDGE_STACK_HOST overrides the probe host (default 127.0.0.1). " +
		"Drachtio / rtpengine / PostgreSQL checks only appear when spec.recording.global_enabled and spec.recording.siprec.enabled are both true. " +
		"Recorder block: SIPREC_RECORDER_BASE_URL (default http://127.0.0.1:3030) or set to none to skip."

	return serviceDashboardResponse{
		CheckedAt:    time.Now().UTC().Format(time.RFC3339),
		SummaryUp:    up,
		SummaryTotal: len(ordered),
		Services:     ordered,
		Log:          snapshotDashboardLog(),
		Note:         note,
		Recorder:     rec,
	}
}

func (s *Server) handleServiceDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	res := s.runServiceDashboard(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}
