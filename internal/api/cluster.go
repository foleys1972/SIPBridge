package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"sipbridge/internal/config"
)

func (s *Server) handleCapacity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	out := s.sipSrv.CapacitySnapshot()
	out["local_instance_id"] = strings.TrimSpace(os.Getenv("SIPBRIDGE_INSTANCE_ID"))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// handleClusterSummary returns local capacity plus optional peer snapshots from spec.servers (GET ?probe=1).
func (s *Server) handleClusterSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	probe := r.URL.Query().Get("probe") == "1" || strings.EqualFold(r.URL.Query().Get("probe"), "true")

	local := s.sipSrv.CapacitySnapshot()
	local["local_instance_id"] = strings.TrimSpace(os.Getenv("SIPBRIDGE_INSTANCE_ID"))
	local["cluster_limits"] = s.sipSrv.ClusterLimits()

	out := map[string]any{
		"local": local,
		"peers": []any{},
	}
	if !probe || s.cm == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	peers := make([]any, 0)
	for _, srv := range s.cm.Current().Spec.Servers {
		row := map[string]any{
			"id":                     srv.ID,
			"name":                   srv.Name,
			"api_base_url":           srv.APIBaseURL,
			"region":                 srv.Region,
			"sip_ingress_uri":        srv.SIPIngressURI,
			"interconnect_sip_uri":   srv.InterconnectSIPURI,
			"capacity_weight":        srv.CapacityWeight,
			"tls_skip_verify":        srv.TLSSkipVerify,
			"capacity":               nil,
			"capacity_error":         "",
			"capacity_latency_ms":    int64(0),
		}
		base := strings.TrimRight(strings.TrimSpace(srv.APIBaseURL), "/")
		if base == "" {
			row["capacity_error"] = "missing api_base_url"
			peers = append(peers, row)
			continue
		}
		u := base + "/v1/capacity"
		start := time.Now()
		client := &http.Client{Timeout: 4 * time.Second}
		if srv.TLSSkipVerify {
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			}
			client.Transport = tr
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			row["capacity_error"] = err.Error()
			peers = append(peers, row)
			continue
		}
		req.Header.Set("User-Agent", "sipbridge-cluster-summary/1")
		resp, err := client.Do(req)
		row["capacity_latency_ms"] = time.Since(start).Milliseconds()
		if err != nil {
			row["capacity_error"] = err.Error()
			peers = append(peers, row)
			continue
		}
		func() {
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				row["capacity_error"] = http.StatusText(resp.StatusCode)
				return
			}
			var cap map[string]any
			if err := json.Unmarshal(b, &cap); err != nil {
				row["capacity_error"] = err.Error()
				return
			}
			row["capacity"] = cap
		}()
		peers = append(peers, row)
	}
	out["peers"] = peers

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// handleClusterConfig returns merged cluster limits (env + spec.cluster) for visibility.
func (s *Server) handleClusterConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if s.cm == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"cluster": config.ClusterLimits{}})
		return
	}
	// Effective limits are runtime on sip server; spec.cluster is partial until merge — expose runtime.
	_ = json.NewEncoder(w).Encode(map[string]any{
		"effective": s.sipSrv.ClusterLimits(),
		"saved":     s.cm.Current().Spec.Cluster,
		"note":      "effective is what the SIP stack uses; saved is spec.cluster from config (merged at startup).",
	})
}
