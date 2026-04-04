package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"sipbridge/internal/config"

	"gopkg.in/yaml.v3"
)

// handleServersV1 serves GET /v1/servers and PUT /v1/servers (inventory of peer SIPBridge API endpoints).
func (s *Server) handleServersV1(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/servers")
	path = strings.Trim(path, "/")
	if path != "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleServersGet(w, r)
	case http.MethodPut:
		s.handleServersPut(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func localInstanceID() string {
	return strings.TrimSpace(os.Getenv("SIPBRIDGE_INSTANCE_ID"))
}

func (s *Server) handleServersGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cm == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"local_instance_id": localInstanceID(),
			"servers":           []config.ManagedServer{},
		})
		return
	}
	probe := r.URL.Query().Get("probe") == "1" || strings.EqualFold(r.URL.Query().Get("probe"), "true")
	list := s.cm.Current().Spec.Servers
	if list == nil {
		list = []config.ManagedServer{}
	}
	if !probe {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"local_instance_id": localInstanceID(),
			"servers":           list,
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	out := make([]map[string]any, 0, len(list))
	for _, srv := range list {
		row := map[string]any{
			"id":              srv.ID,
			"name":            srv.Name,
			"api_base_url":    srv.APIBaseURL,
			"region":          srv.Region,
			"tls_skip_verify": srv.TLSSkipVerify,
		}
		ok, ms, errStr := probeServerHealth(ctx, srv.APIBaseURL, srv.TLSSkipVerify)
		row["probe"] = map[string]any{
			"ok":         ok,
			"latency_ms": ms,
			"error":      errStr,
		}
		out = append(out, row)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"local_instance_id": localInstanceID(),
		"servers":           out,
	})
}

func probeServerHealth(ctx context.Context, baseURL string, tlsSkipVerify bool) (ok bool, latencyMs int64, errMsg string) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return false, 0, "empty api_base_url"
	}
	u := base + "/healthz"
	client := &http.Client{Timeout: 3 * time.Second}
	if tlsSkipVerify {
		tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // operator-controlled per server entry
		client.Transport = tr
	}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, 0, err.Error()
	}
	resp, err := client.Do(req)
	latencyMs = time.Since(start).Milliseconds()
	if err != nil {
		return false, latencyMs, err.Error()
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return false, latencyMs, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return true, latencyMs, ""
}

func (s *Server) handleServersPut(w http.ResponseWriter, r *http.Request) {
	if s.cm == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Servers []config.ManagedServer `json:"servers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "invalid JSON"})
		return
	}
	if req.Servers == nil {
		req.Servers = []config.ManagedServer{}
	}
	if err := config.ValidateManagedServers(req.Servers); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	cur := s.cm.Current()
	cur.Spec.Servers = req.Servers
	yamlBytes, err := yaml.Marshal(&cur)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if _, err := s.cm.ApplyYAML(yamlBytes); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"servers": s.cm.Current().Spec.Servers,
	})
}
